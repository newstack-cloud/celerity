// Package awscreds resolves database credentials on AWS: Secrets Manager for
// stored credentials, RDS IAM for short-lived tokens.
//
// It is the AWS implementation of dbconn.CredentialSource. A provider added
// later supplies its own, and nothing above this line changes.
package awscreds

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn"
)

// Source resolves credentials from Secrets Manager and RDS IAM.
type Source struct{}

// New returns the AWS credential source.
func New() *Source { return &Source{} }

// Secret is the credential JSON in a Secrets Manager secret.
//
// RDS-managed secrets carry the full connection detail, where the secrets
// Celerity generates carry username and password only. Connection detail is
// parsed but treated as a fallback, because an endpoint derived from deployed
// state points at the proxy while the secret names the database behind it.
type Secret struct {
	Username string      `json:"username"`
	Password string      `json:"password"`
	Host     string      `json:"host"`
	Port     json.Number `json:"port"`
	DBName   string      `json:"dbname"`
}

// Resolve reads a credential from Secrets Manager. The reference is a secret
// ARN or name.
func (s *Source) Resolve(ctx context.Context, reference string) (dbconn.Credentials, error) {
	secret, err := FetchSecret(ctx, reference)
	if err != nil {
		return dbconn.Credentials{}, err
	}
	return dbconn.Credentials{Username: secret.Username, Password: secret.Password}, nil
}

// Token builds a short-lived RDS IAM authentication token to use in place of a
// password.
func (s *Source) Token(
	ctx context.Context,
	endpoint dbconn.Endpoint,
	user string,
	region string,
) (dbconn.Credentials, error) {
	if user == "" {
		return dbconn.Credentials{}, fmt.Errorf(
			"IAM authentication needs a database user, and none was resolved")
	}

	opts := []func(*config.LoadOptions) error{}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return dbconn.Credentials{}, fmt.Errorf("loading AWS config: %w", err)
	}
	if region == "" {
		region = awsCfg.Region
	}
	if region == "" {
		return dbconn.Credentials{}, fmt.Errorf(
			"could not determine an AWS region for IAM authentication")
	}

	token, err := auth.BuildAuthToken(
		ctx, net.JoinHostPort(endpoint.Host, endpoint.Port), region, user, awsCfg.Credentials)
	if err != nil {
		return dbconn.Credentials{}, fmt.Errorf("building RDS IAM auth token: %w", err)
	}
	return dbconn.Credentials{Username: user, Password: token}, nil
}

// FetchSecret reads and parses a Secrets Manager secret, including the
// connection detail an RDS-managed secret carries.
//
// The region comes from the ARN so this works from a machine whose default
// region is somewhere else, which is the normal case for an operator with
// several environments.
func FetchSecret(ctx context.Context, secretARN string) (*Secret, error) {
	opts := []func(*config.LoadOptions) error{}
	if region := RegionFromARN(secretARN); region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(awsCfg)
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: &secretARN})
	if err != nil {
		return nil, fmt.Errorf("fetching secret %s: %w", secretARN, err)
	}
	if out.SecretString == nil {
		return nil, fmt.Errorf("secret %s has no string value", secretARN)
	}

	var secret Secret
	if err := json.Unmarshal([]byte(*out.SecretString), &secret); err != nil {
		return nil, fmt.Errorf("parsing secret %s value as JSON: %w", secretARN, err)
	}
	if secret.Username == "" || secret.Password == "" {
		return nil, fmt.Errorf("secret %s is missing username/password", secretARN)
	}
	return &secret, nil
}

// RegionFromARN extracts the region from an ARN
// (arn:aws:secretsmanager:<region>:<account>:secret:<name>).
func RegionFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

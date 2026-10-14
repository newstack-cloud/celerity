package deploydb

import (
	"context"
	"fmt"

	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn"
	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn/awscreds"
)

// Concrete AWS resource types the transformer emits for a celerity/sqlDatabase.
const (
	typeDBInstance = "aws/rds/dbInstance"
	typeDBCluster  = "aws/rds/dbCluster"
	typeDBProxy    = "aws/rds/dbProxy"
	typeSecret     = "aws/secretsmanager/secret"

	defaultPort     = "5432"
	defaultDatabase = "postgres"
)

// awsResolver resolves connections for the aws / aws-serverless deploy targets.
type awsResolver struct{}

func (awsResolver) Resolve(
	ctx context.Context,
	instance *state.InstanceState,
	input ResolveInput,
) (*ResolvedConnection, error) {
	sqlDatabaseName := input.SQLDatabaseName
	target, err := resolveAWSTarget(instance, sqlDatabaseName, input.FallbackDatabase)
	if err != nil {
		return nil, err
	}

	displayTarget := fmt.Sprintf("%s:%s/%s", target.Host, target.Port, target.Database)

	// Connection building is shared with the maintenance runner (see
	// internal/dbconn), because the two paths reach the same databases and a
	// difference between them would only ever show up on deployed
	// infrastructure.
	endpoint := dbconn.Endpoint{
		Engine:   input.Engine,
		Host:     target.Host,
		Port:     target.Port,
		Database: target.Database,
		// A managed database always speaks TLS, and IAM authentication requires
		// it.
		TLS: true,
	}
	source := awscreds.New()

	var creds dbconn.Credentials
	if target.IAMAuth {
		creds, err = source.Token(ctx, endpoint, target.DBUser, target.Region)
		if err != nil {
			return nil, fmt.Errorf(
				"authenticating to database %q by IAM: %w; run migrations with --url if this cannot be resolved",
				sqlDatabaseName, err)
		}
	} else {
		if target.SecretARN == "" {
			return nil, fmt.Errorf("no credential secret found in state for database %q", sqlDatabaseName)
		}
		secret, secretErr := awscreds.FetchSecret(ctx, target.SecretARN)
		if secretErr != nil {
			return nil, secretErr
		}
		creds = dbconn.Credentials{Username: secret.Username, Password: secret.Password}
		// State-derived host/port point at the proxy and win; the secret's own
		// connection detail names the database behind it, and is only a
		// fallback.
		endpoint.Host = firstNonEmpty(endpoint.Host, secret.Host)
		endpoint.Port = firstNonEmpty(endpoint.Port, secret.Port.String())
		endpoint.Database = firstNonEmpty(endpoint.Database, secret.DBName, defaultDatabase)
	}

	connStr, err := dbconn.URL(endpoint, creds)
	if err != nil {
		return nil, err
	}
	return &ResolvedConnection{ConnectionString: connStr, Target: displayTarget}, nil
}

// The AWS-specific connection information extracted from deployed state, before
// credentials are fetched.
type connectionTarget struct {
	Host      string
	Port      string
	Database  string
	SecretARN string
	IAMAuth   bool
	// DBUser and Region are used for IAM authentication (no secret involved).
	DBUser string
	Region string
}

// Finds the concrete AWS resources generated for a celerity/sqlDatabase and
// derives the connection target. It makes no AWS calls.
func resolveAWSTarget(instance *state.InstanceState, sqlDatabaseName, fallbackDatabase string) (*connectionTarget, error) {
	if instance == nil {
		return nil, fmt.Errorf("no instance state available")
	}

	var proxy, cluster, dbInstance, secret *state.ResourceState
	for _, resource := range instance.Resources {
		if abstractName(resource) != sqlDatabaseName {
			continue
		}
		switch resource.Type {
		case typeDBProxy:
			proxy = resource
		case typeDBCluster:
			cluster = resource
		case typeDBInstance:
			dbInstance = resource
		case typeSecret:
			// Migrations run as the owner. The application user's secret carries
			// the same abstract name, so it has to be filtered out here or the
			// migration path connects as a role with no rights to alter anything.
			if secretRole(resource) != secretRoleAppUser {
				secret = resource
			}
		}
	}

	db, dbNameField := cluster, "databaseName"
	if db == nil {
		db, dbNameField = dbInstance, "dbName"
	}
	if db == nil && proxy == nil {
		return nil, fmt.Errorf("no deployed RDS database found for %q in instance state", sqlDatabaseName)
	}

	target := &connectionTarget{Port: defaultPort, Database: fallbackDatabase}

	// Prefer the proxy endpoint (a flat string); fall back to the DB endpoint.
	if proxy != nil {
		target.Host = nodeString(proxy.SpecData, "endpoint")
	}
	if db != nil {
		if target.Host == "" {
			target.Host = nodeString(db.SpecData, "endpoint", "address")
		}
		if port := firstNonEmpty(nodeString(db.SpecData, "endpoint", "port"), nodeString(db.SpecData, "port")); port != "" {
			target.Port = port
		}
		if name := nodeString(db.SpecData, dbNameField); name != "" {
			target.Database = name
		}
		target.IAMAuth = nodeString(db.SpecData, "enableIAMDatabaseAuthentication") == "true" ||
			nodeString(db.SpecData, "masterUserAuthenticationType") == "iam-db-auth"
		target.DBUser = nodeString(db.SpecData, "masterUsername")
		target.Region = awscreds.RegionFromARN(nodeString(db.SpecData, "arn"))
	}
	if target.Database == "" {
		target.Database = defaultDatabase
	}

	// Password auth: prefer the transformer-created secret; fall back to an
	// RDS-managed master-user secret on the database resource.
	if secret != nil {
		target.SecretARN = nodeString(secret.SpecData, "id")
	}
	if target.SecretARN == "" && db != nil {
		target.SecretARN = nodeString(db.SpecData, "masterUserSecret", "secretArn")
	}

	if target.Host == "" {
		return nil, fmt.Errorf("could not resolve a database endpoint for %q from instance state", sqlDatabaseName)
	}
	return target, nil
}

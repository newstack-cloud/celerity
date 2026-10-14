package appconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"go.uber.org/zap"
)

// ssmAPI is the subset of the SSM client used by this store.
type ssmAPI interface {
	GetParameter(
		ctx context.Context,
		params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.GetParameterOutput, error)
	PutParameter(
		ctx context.Context,
		params *ssm.PutParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.PutParameterOutput, error)
	DeleteParameter(
		ctx context.Context,
		params *ssm.DeleteParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.DeleteParameterOutput, error)
	GetParametersByPath(
		ctx context.Context,
		params *ssm.GetParametersByPathInput,
		optFns ...func(*ssm.Options),
	) (*ssm.GetParametersByPathOutput, error)
}

type parameterStoreStore struct {
	client        ssmAPI
	pathPrefix    string
	plaintextKeys map[string]bool
	logger        *zap.Logger
}

func (p *parameterStoreStore) Get(ctx context.Context, key string) (*ConfigEntry, error) {
	paramName := p.pathPrefix + key

	out, err := p.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("getting parameter %q: %w", paramName, err)
	}

	return &ConfigEntry{
		Key:       key,
		Value:     aws.ToString(out.Parameter.Value),
		Sensitive: out.Parameter.Type == ssmtypes.ParameterTypeSecureString,
	}, nil
}

func (p *parameterStoreStore) Set(
	ctx context.Context,
	key, value string,
	sensitive bool,
) error {
	paramName := p.pathPrefix + key
	paramType := ssmtypes.ParameterTypeSecureString
	if p.plaintextKeys[key] && !sensitive {
		paramType = ssmtypes.ParameterTypeString
	}

	_, err := p.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(paramName),
		Value:     aws.String(value),
		Type:      paramType,
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("setting parameter %q: %w", paramName, err)
	}
	return nil
}

func (p *parameterStoreStore) Delete(ctx context.Context, key string) error {
	paramName := p.pathPrefix + key

	_, err := p.client.DeleteParameter(ctx, &ssm.DeleteParameterInput{
		Name: aws.String(paramName),
	})
	if err != nil {
		return fmt.Errorf("deleting parameter %q: %w", paramName, err)
	}
	return nil
}

func (p *parameterStoreStore) List(ctx context.Context) ([]ConfigEntry, error) {
	var entries []ConfigEntry
	var nextToken *string

	for {
		out, err := p.client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
			Path:           aws.String(p.pathPrefix),
			Recursive:      aws.Bool(true),
			WithDecryption: aws.Bool(true),
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing parameters under %q: %w", p.pathPrefix, err)
		}

		for _, param := range out.Parameters {
			key := strings.TrimPrefix(aws.ToString(param.Name), p.pathPrefix)
			entries = append(entries, ConfigEntry{
				Key:       key,
				Value:     aws.ToString(param.Value),
				Sensitive: param.Type == ssmtypes.ParameterTypeSecureString,
			})
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return entries, nil
}

func (p *parameterStoreStore) Import(
	ctx context.Context,
	entries []ConfigEntry,
) error {
	for _, e := range entries {
		if err := p.Set(ctx, e.Key, e.Value, e.Sensitive); err != nil {
			return err
		}
	}
	return nil
}

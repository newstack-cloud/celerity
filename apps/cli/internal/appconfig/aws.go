package appconfig

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func newAWSConfigStore(
	ctx context.Context,
	info StoreInfo,
	opts ProviderOpts,
) (ConfigStore, error) {
	cfgOpts := []func(*awsconfig.LoadOptions) error{}
	if opts.Region != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithRegion(opts.Region))
	}
	if opts.Profile != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithSharedConfigProfile(opts.Profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	switch info.StoreKind {
	case StoreKindSecretsManager:
		return &secretsManagerStore{
			client:   secretsmanager.NewFromConfig(cfg),
			secretID: info.StoreName,
			logger:   opts.Logger,
		}, nil
	case StoreKindParameterStore:
		return &parameterStoreStore{
			client:        ssm.NewFromConfig(cfg),
			pathPrefix:    "/" + info.StoreName + "/",
			plaintextKeys: info.PlaintextKeys,
			logger:        opts.Logger,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported store kind %q", info.StoreKind)
	}
}

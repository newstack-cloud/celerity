package deployconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConvertTestSuite struct {
	suite.Suite
}

func TestConvertTestSuite(t *testing.T) {
	suite.Run(t, new(ConvertTestSuite))
}

func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_aws_serverless() {
	cfg := &AppDeployConfig{
		AppName: "orders-service",
		DeployTarget: DeployTarget{
			Name:   "aws-serverless",
			AppEnv: "production",
			Config: map[string]any{
				"aws.accessKeyId":     "AKIA...",
				"aws.secretAccessKey": "secret",
				"aws.region":          "us-east-1",
			},
		},
		BlueprintVariables: map[string]any{
			"region": "us-east-1",
		},
	}

	blCfg, err := ConvertToBluelinkFormat(cfg)
	s.NoError(err)

	// Provider config should have credentials with the "aws." namespace prefix
	// stripped, since the provider plugin uses bare field names.
	s.Equal("AKIA...", blCfg.Providers["aws"]["accessKeyId"])
	s.Equal("secret", blCfg.Providers["aws"]["secretAccessKey"])
	s.Equal("us-east-1", blCfg.Providers["aws"]["region"])
	s.NotContains(blCfg.Providers["aws"], "aws.accessKeyId")

	// Deploy target is surfaced as a context variable (read by the transformer
	// via the SDK's "deployTarget" key), not as transformer config.
	s.Equal("aws-serverless", blCfg.ContextVariables["deployTarget"])
	s.NotContains(blCfg.Transformers["celerity"], "deployTarget")

	// Blueprint variables pass through
	s.Equal("us-east-1", blCfg.BlueprintVariables["region"])

	// AppEnv should be a context variable
	s.Equal("production", blCfg.ContextVariables["appEnv"])

	// AppName should be surfaced as the celerity.appName context variable
	s.Equal("orders-service", blCfg.ContextVariables["celerity.appName"])

	// Default plugin IDs, pinned to the CLI's tested version constraints.
	// The transformer is unconstrained for now (see defaultPluginConstraints).
	s.Equal("^0.4.0", blCfg.Dependencies["newstack-cloud/aws"])
	s.Contains(blCfg.Dependencies, "newstack-cloud/celerity")
	s.Equal("", blCfg.Dependencies["newstack-cloud/celerity"])
}

func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_splits_transformer_config() {
	cfg := &AppDeployConfig{
		DeployTarget: DeployTarget{
			Name: "aws",
			Config: map[string]any{
				"aws.accessKeyId":                "AKIA...",
				"aws.sqs.messageRetentionPeriod": 345600,
				"aws.sqs.myQueue.maxMessageSize": 65536,
				"aws.ecs.some.setting":           "value",
			},
		},
	}

	blCfg, err := ConvertToBluelinkFormat(cfg)
	s.NoError(err)

	// Provider config: only credentials, with the namespace prefix stripped
	s.Equal("AKIA...", blCfg.Providers["aws"]["accessKeyId"])
	s.NotContains(blCfg.Providers["aws"], "aws.sqs.messageRetentionPeriod")

	// Transformer config: resource-specific settings
	s.Equal(345600, blCfg.Transformers["celerity"]["aws.sqs.messageRetentionPeriod"])
	s.Equal(65536, blCfg.Transformers["celerity"]["aws.sqs.myQueue.maxMessageSize"])
	s.Equal("value", blCfg.Transformers["celerity"]["aws.ecs.some.setting"])
}

// The transformer opens its own session against the same account, so how to
// reach the account goes to both plugins. A managed celerity/vpc cannot be
// emitted without a region, and the transformer reads the build manifest from
// object storage under these same credentials.
func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_shares_session_config_with_transformer() {
	cfg := &AppDeployConfig{
		AppName: "test-app",
		DeployTarget: DeployTarget{
			Name: "aws-serverless",
			Config: map[string]any{
				"aws.profile":            "some-profile",
				"aws.region":             "eu-west-2",
				"aws.assumeRole.roleArn": "arn:aws:iam::123456789012:role/deploy",
			},
		},
	}

	blCfg, err := ConvertToBluelinkFormat(cfg)
	s.NoError(err)

	// The provider matches on the bare key.
	s.Equal("some-profile", blCfg.Providers["aws"]["profile"])
	s.Equal("eu-west-2", blCfg.Providers["aws"]["region"])
	s.Equal("arn:aws:iam::123456789012:role/deploy", blCfg.Providers["aws"]["assumeRole.roleArn"])

	// The transformer matches on the namespaced key.
	s.Equal("some-profile", blCfg.Transformers["celerity"]["aws.profile"])
	s.Equal("eu-west-2", blCfg.Transformers["celerity"]["aws.region"])
	s.Equal(
		"arn:aws:iam::123456789012:role/deploy",
		blCfg.Transformers["celerity"]["aws.assumeRole.roleArn"],
	)
}

// A plugin rejects configuration fields it does not declare, so a key only the
// provider declares must not be copied across.
func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_keeps_provider_only_config_off_the_transformer() {
	cfg := &AppDeployConfig{
		AppName: "test-app",
		DeployTarget: DeployTarget{
			Name: "aws-serverless",
			Config: map[string]any{
				"aws.region":                         "eu-west-2",
				"aws.endpoints":                      "s3=http://localhost:4566",
				"aws.ec2MetadataServiceEndpointMode": "IPv6",
			},
		},
	}

	blCfg, err := ConvertToBluelinkFormat(cfg)
	s.NoError(err)

	s.Equal("s3=http://localhost:4566", blCfg.Providers["aws"]["endpoints"])
	s.Equal("IPv6", blCfg.Providers["aws"]["ec2MetadataServiceEndpointMode"])

	s.NotContains(blCfg.Transformers["celerity"], "aws.endpoints")
	s.NotContains(blCfg.Transformers["celerity"], "aws.ec2MetadataServiceEndpointMode")
}

func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_gcloud() {
	cfg := &AppDeployConfig{
		DeployTarget: DeployTarget{
			Name: "gcloud-serverless",
			Config: map[string]any{
				"gcloud.project": "my-project",
				"gcloud.pubsub.topicAsQueue.messageRetentionPeriod": "7d",
			},
		},
	}

	blCfg, err := ConvertToBluelinkFormat(cfg)
	s.NoError(err)

	s.Equal("my-project", blCfg.Providers["gcloud"]["project"])
	s.Equal("7d", blCfg.Transformers["celerity"]["gcloud.pubsub.topicAsQueue.messageRetentionPeriod"])
	s.Equal("gcloud-serverless", blCfg.ContextVariables["deployTarget"])
	s.NotContains(blCfg.Transformers["celerity"], "deployTarget")
	s.Equal("^0.1.0", blCfg.Dependencies["newstack-cloud/gcloud"])
}

func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_default_plugin_ids() {
	cfg := &AppDeployConfig{
		DeployTarget: DeployTarget{
			Name: "aws",
		},
	}

	blCfg, err := ConvertToBluelinkFormat(cfg)
	s.NoError(err)

	s.Equal("^0.4.0", blCfg.Dependencies["newstack-cloud/aws"])
	s.Contains(blCfg.Dependencies, "newstack-cloud/celerity")
	s.Equal("", blCfg.Dependencies["newstack-cloud/celerity"])
}

func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_unsupported_target() {
	cfg := &AppDeployConfig{
		DeployTarget: DeployTarget{
			Name: "unsupported-target",
		},
	}

	_, err := ConvertToBluelinkFormat(cfg)
	s.Error(err)
	s.Contains(err.Error(), "unsupported deploy target")
}

func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_context_variables_pass_through() {
	cfg := &AppDeployConfig{
		DeployTarget: DeployTarget{
			Name: "aws",
		},
		ContextVariables: map[string]any{
			"myKey": "myValue",
		},
	}

	blCfg, err := ConvertToBluelinkFormat(cfg)
	s.NoError(err)

	s.Equal("myValue", blCfg.ContextVariables["myKey"])
}

func (s *ConvertTestSuite) Test_ReadAppDeployConfig_jsonc() {
	content := `{
  // This is a comment
  "appName": "orders-service",
  "deployTarget": {
    "name": "aws",
    "appEnv": "production",
    "config": {
      "accessKeyId": "test-key",
    },
  },
  "blueprintVariables": {
    "region": "eu-west-1",
  }
}`
	tmpDir := s.T().TempDir()
	path := filepath.Join(tmpDir, "app.deploy.jsonc")
	s.NoError(os.WriteFile(path, []byte(content), 0644))

	cfg, err := ReadAppDeployConfig(path)
	s.NoError(err)
	s.Equal("orders-service", cfg.AppName)
	s.Equal("aws", cfg.DeployTarget.Name)
	s.Equal("production", cfg.DeployTarget.AppEnv)
	s.Equal("test-key", cfg.DeployTarget.Config["accessKeyId"])
	s.Equal("eu-west-1", cfg.BlueprintVariables["region"])
}

func (s *ConvertTestSuite) Test_ReadAppDeployConfig_missing_deploy_target() {
	content := `{"appName": "orders-service", "blueprintVariables": {}}`
	tmpDir := s.T().TempDir()
	path := filepath.Join(tmpDir, "app.deploy.jsonc")
	s.NoError(os.WriteFile(path, []byte(content), 0644))

	_, err := ReadAppDeployConfig(path)
	s.Error(err)
	s.Contains(err.Error(), "missing deployTarget.name")
}

func (s *ConvertTestSuite) Test_ReadAppDeployConfig_missing_app_name() {
	content := `{"deployTarget": {"name": "aws-serverless"}}`
	tmpDir := s.T().TempDir()
	path := filepath.Join(tmpDir, "app.deploy.jsonc")
	s.NoError(os.WriteFile(path, []byte(content), 0644))

	_, err := ReadAppDeployConfig(path)
	s.Error(err)
	s.Contains(err.Error(), "missing appName")
}

func (s *ConvertTestSuite) Test_ReadAppDeployConfig_invalid_app_name() {
	content := `{"appName": "orders/service", "deployTarget": {"name": "aws-serverless"}}`
	tmpDir := s.T().TempDir()
	path := filepath.Join(tmpDir, "app.deploy.jsonc")
	s.NoError(os.WriteFile(path, []byte(content), 0644))

	_, err := ReadAppDeployConfig(path)
	s.Error(err)
	s.Contains(err.Error(), "invalid appName")
}

func (s *ConvertTestSuite) Test_stripJSONCComments_line_comments() {
	input := `{
  // comment
  "key": "value" // trailing comment
}`
	result := stripJSONCComments(input)
	s.NotContains(result, "comment")
	s.Contains(result, `"key": "value"`)
}

func (s *ConvertTestSuite) Test_stripJSONCComments_block_comments() {
	input := `{
  /* block comment */
  "key": "value"
}`
	result := stripJSONCComments(input)
	s.NotContains(result, "block comment")
	s.Contains(result, `"key": "value"`)
}

func (s *ConvertTestSuite) Test_stripJSONCComments_preserves_strings() {
	input := `{"key": "value // not a comment"}`
	result := stripJSONCComments(input)
	s.Contains(result, "// not a comment")
}

func (s *ConvertTestSuite) Test_stripJSONCComments_trailing_commas() {
	input := `{
  "key": "value",
}`
	result := stripJSONCComments(input)
	s.NotContains(result, ",\n}")
}

// The vpc, rds, aurora and elasticache prefixes must reach the transformer that
// reads them; sent to the provider they fail validation as unexpected fields.
func (s *ConvertTestSuite) Test_ConvertToBluelinkFormat_routes_vpc_database_and_cache_config() {
	cfg := &AppDeployConfig{
		DeployTarget: DeployTarget{
			Name: "aws",
			Config: map[string]any{
				"aws.vpc.appVpc.cidrBlock":              "10.42.0.0/16",
				"aws.vpc.enableDNSHostnames":            true,
				"aws.rds.appDb.instanceClass":           "db.r6g.large",
				"aws.rds.appDb.allocatedStorage":        200,
				"aws.aurora.appDb.enabled":              true,
				"aws.aurora.appDb.minACU":               0.5,
				"aws.elasticache.appCache.nodeType":     "cache.t4g.micro",
				"aws.elasticache.snapshotRetentionDays": 7,
			},
		},
	}

	blCfg, err := ConvertToBluelinkFormat(cfg)
	s.Require().NoError(err)

	transformerCfg := blCfg.Transformers["celerity"]
	s.Equal("10.42.0.0/16", transformerCfg["aws.vpc.appVpc.cidrBlock"])
	s.Equal(true, transformerCfg["aws.vpc.enableDNSHostnames"])
	s.Equal("db.r6g.large", transformerCfg["aws.rds.appDb.instanceClass"])
	s.Equal(200, transformerCfg["aws.rds.appDb.allocatedStorage"])
	s.Equal(true, transformerCfg["aws.aurora.appDb.enabled"])
	s.Equal(0.5, transformerCfg["aws.aurora.appDb.minACU"])
	s.Equal("cache.t4g.micro", transformerCfg["aws.elasticache.appCache.nodeType"])
	s.Equal(7, transformerCfg["aws.elasticache.snapshotRetentionDays"])

	// None of them may reach the provider, in either key shape.
	providerCfg := blCfg.Providers["aws"]
	for _, key := range []string{
		"vpc.appVpc.cidrBlock",
		"aws.vpc.appVpc.cidrBlock",
		"rds.appDb.instanceClass",
		"aurora.appDb.minACU",
		"elasticache.appCache.nodeType",
	} {
		s.NotContains(providerCfg, key)
	}
}

// Package deployconfig handles reading Celerity app deploy configuration
// and converting it to the Bluelink deploy configuration format.
package deployconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
)

// AppDeployConfig represents the structure of app.deploy.jsonc.
type AppDeployConfig struct {
	// AppName is the project name. It is a first-class authoring item used for
	// resource naming and configuration-store paths (e.g. the SSM path prefix
	// "/celerity/<appName>/<configName>"). It is surfaced to the transform as
	// the "celerity.appName" context variable.
	AppName string `json:"appName"`
	// DeployTarget holds the target environment and its configuration.
	DeployTarget DeployTarget `json:"deployTarget"`
	// ContextVariables are additional context variables passed to the deploy engine.
	ContextVariables map[string]any `json:"contextVariables,omitempty"`
	// BlueprintVariables are blueprint variable overrides.
	BlueprintVariables map[string]any `json:"blueprintVariables,omitempty"`
	// HandlerBuild describes how to build the handlers executable for an OS-only
	// runtime, where the application is in a language the CLI has no toolchain
	// of its own for.
	//
	// This is not forwarded to the deploy engine. ConvertToBluelinkFormat
	// builds its output field by field, and this one describes a local build
	// step rather than anything the engine acts on.
	HandlerBuild *handlertoolchain.Config `json:"handlerBuild,omitempty"`
}

// Constrains appName to characters safe for resource names and hierarchical
// store paths, so no slashes, spaces or leading punctuation.
var appNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-_]*$`)

// DeployTarget holds the target environment and its configuration.
type DeployTarget struct {
	Name   string         `json:"name"`
	AppEnv string         `json:"appEnv,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

// BluelinkDeployConfig is the Bluelink deploy configuration format
// expected by the deploy engine.
type BluelinkDeployConfig struct {
	Providers          map[string]map[string]any `json:"providers,omitempty"`
	Transformers       map[string]map[string]any `json:"transformers,omitempty"`
	ContextVariables   map[string]any            `json:"contextVariables,omitempty"`
	BlueprintVariables map[string]any            `json:"blueprintVariables,omitempty"`
	Dependencies       map[string]string         `json:"dependencies,omitempty"`
}

var deployTargetToProvider = map[string]string{
	"aws":               "aws",
	"aws-serverless":    "aws",
	"gcloud":            "gcloud",
	"gcloud-serverless": "gcloud",
	"azure":             "azure",
	"azure-serverless":  "azure",
}

var defaultPluginIDs = map[string]string{
	"aws":    "newstack-cloud/aws",
	"gcloud": "newstack-cloud/gcloud",
	"azure":  "newstack-cloud/azure",
}

const defaultTransformerPluginID = "newstack-cloud/celerity"

// Pins the provider and transformer plugin versions this CLI release is built
// and tested against, keyed by plugin ID.
//
// Constraints use the caret (^major.minor.patch) form, so the deploy engine
// accepts any installed version with the same major and an equal-or-higher
// minor. Non-breaking plugin upgrades flow through automatically while a
// breaking major bump is gated on upgrading the CLI. Compatibility is a
// property of the CLI release, not the Celerity app, so app developers never
// specify plugin versions.
//
// Bump these when the CLI is validated against a new plugin major/minor.
var defaultPluginConstraints = map[string]string{
	"newstack-cloud/aws":    "^0.4.0",
	"newstack-cloud/gcloud": "^0.1.0",
	"newstack-cloud/azure":  "^0.1.0",
	// TODO: pin once the celerity transformer cuts its first release. Local
	// dev builds report 0.0.0-dev, which fails any caret constraint, so the
	// transformer is left unconstrained (any installed version) for now.
	"newstack-cloud/celerity": "",
}

// An unknown plugin yields an empty string, which the deploy engine treats as
// any installed version.
func pluginVersionConstraint(pluginID string) string {
	return defaultPluginConstraints[pluginID]
}

// Config key prefixes, keyed by provider namespace, marking a value as
// resource-specific deployment configuration handled by the Celerity
// transformer rather than the provider.
//
// A prefix omitted here is sent to the provider, which does not declare it, so
// the key fails validation rather than reaching the plugin that reads it. Whether
// a key is valid is decided by the receiving plugin's config definition, not by
// this list.
var transformerConfigPrefixes = map[string][]string{
	"aws": {
		"aws.sqs.", "aws.sns.", "aws.dynamodb.", "aws.s3.", "aws.lambda.",
		"aws.apigateway.", "aws.ecs.", "aws.eks.",
		"aws.vpc.", "aws.rds.", "aws.aurora.", "aws.elasticache.",
	},
	"gcloud": {"gcloud.pubsub.", "gcloud.cloudfunctions.", "gcloud.cloudrun.", "gcloud.firestore.", "gcloud.storage."},
	"azure":  {"azure.servicebus.", "azure.functions.", "azure.container.", "azure.cosmosdb.", "azure.storage."},
}

// The keys, keyed by provider namespace, that describe how to reach the cloud
// account rather than what to deploy into it. They go to the provider and the
// transformer both.
//
// The transformer opens its own session against the same account, reading the
// build manifest from object storage, and a managed celerity/vpc cannot be
// emitted at all without a region. Routing these to the provider alone leaves
// the transformer unable to resolve credentials or a region, which surfaces as
// an emit failure rather than a configuration error.
//
// This is a fixed list rather than every key that is not resource-specific,
// because a plugin rejects configuration fields it does not declare. Provider-
// only keys such as custom service endpoints and the EC2 metadata service
// address would therefore fail validation if they were copied across. Extend
// this only with keys both plugins declare.
var sharedSessionConfigKeys = map[string][]string{
	"aws": {
		"aws.accessKeyId",
		"aws.secretAccessKey",
		"aws.sessionToken",
		"aws.profile",
		"aws.region",
		"aws.assumeRole.roleArn",
		"aws.assumeRole.sessionName",
		"aws.assumeRole.externalId",
		"aws.assumeRole.duration",
		"aws.assumeRoleWithWebIdentity.roleArn",
		"aws.assumeRoleWithWebIdentity.sessionName",
		"aws.assumeRoleWithWebIdentity.webIdentityTokenFile",
		"aws.assumeRoleWithWebIdentity.duration",
		"aws.sharedConfigFiles",
		"aws.sharedCredentialsFiles",
		"aws.customCABundle",
		"aws.httpProxy",
		"aws.httpsProxy",
		"aws.insecure",
		"aws.maxRetries",
		"aws.retryMode",
		"aws.useDualStackEndpoint",
		"aws.useFIPSEndpoint",
	},
}

// ReadAppDeployConfig reads and parses an app deploy config file.
// Supports JSONC (strips comments before parsing).
func ReadAppDeployConfig(path string) (*AppDeployConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading app deploy config %s: %w", path, err)
	}

	cleaned := stripJSONCComments(string(data))

	var cfg AppDeployConfig
	if err := json.Unmarshal([]byte(cleaned), &cfg); err != nil {
		return nil, fmt.Errorf("parsing app deploy config %s: %w", path, err)
	}

	if cfg.DeployTarget.Name == "" {
		return nil, fmt.Errorf("app deploy config %s missing deployTarget.name", path)
	}

	if cfg.AppName == "" {
		return nil, fmt.Errorf("app deploy config %s missing appName", path)
	}

	if !appNamePattern.MatchString(cfg.AppName) {
		return nil, fmt.Errorf(
			"app deploy config %s has invalid appName %q: must start with a letter or digit and "+
				"contain only letters, digits, hyphens, and underscores",
			path, cfg.AppName,
		)
	}

	return &cfg, nil
}

// ConvertToBluelinkFormat converts an app deploy config to the Bluelink
// deploy configuration format expected by the deploy engine.
func ConvertToBluelinkFormat(cfg *AppDeployConfig) (*BluelinkDeployConfig, error) {
	providerNamespace, ok := deployTargetToProvider[cfg.DeployTarget.Name]
	if !ok {
		return nil, fmt.Errorf(
			"unsupported deploy target %q, must be one of: aws, aws-serverless, gcloud, gcloud-serverless, azure, azure-serverless",
			cfg.DeployTarget.Name,
		)
	}

	providerPluginID := defaultPluginIDs[providerNamespace]
	transformerPluginID := defaultTransformerPluginID

	providerConfig, transformerConfig := splitConfig(cfg.DeployTarget.Config, providerNamespace)

	blConfig := &BluelinkDeployConfig{
		Providers: map[string]map[string]any{
			providerNamespace: providerConfig,
		},
		Transformers: map[string]map[string]any{
			"celerity": transformerConfig,
		},
		BlueprintVariables: cfg.BlueprintVariables,
		ContextVariables:   cfg.ContextVariables,
		Dependencies: map[string]string{
			providerPluginID:    pluginVersionConstraint(providerPluginID),
			transformerPluginID: pluginVersionConstraint(transformerPluginID),
		},
	}

	if blConfig.ContextVariables == nil {
		blConfig.ContextVariables = map[string]any{}
	}

	// The deploy target is read by the Celerity transformer from a context
	// variable (the transformerv1 SDK's default "deployTarget" key), not from
	// transformer config, so it must be surfaced as a context variable.
	blConfig.ContextVariables["deployTarget"] = cfg.DeployTarget.Name

	// Surface the first-class appName as the "celerity.appName" context variable
	// the transformer reads.
	blConfig.ContextVariables["celerity.appName"] = cfg.AppName

	if cfg.DeployTarget.AppEnv != "" {
		blConfig.ContextVariables["appEnv"] = cfg.DeployTarget.AppEnv
	}

	return blConfig, nil
}

// Keys that match a transformer config prefix, such as
// "aws.sqs.messageRetentionPeriod", go to transformer config with the key
// preserved as-is, since the transformer expects the full dotted key.
//
// All other keys go to provider config with the provider namespace prefix
// stripped, so "aws.region" becomes "region", because the provider plugin's
// config definition uses bare field names within its own namespace.
func splitConfig(config map[string]any, providerNamespace string) (providerCfg, transformerCfg map[string]any) {
	providerCfg = map[string]any{}
	transformerCfg = map[string]any{}

	if config == nil {
		return providerCfg, transformerCfg
	}

	prefixes := transformerConfigPrefixes[providerNamespace]
	sharedKeys := sharedSessionConfigKeys[providerNamespace]

	for key, value := range config {
		if isTransformerConfigKey(key, prefixes) {
			transformerCfg[key] = value
			continue
		}

		providerCfg[strings.TrimPrefix(key, providerNamespace+".")] = value

		// The provider matches on the bare key, the transformer on the
		// namespaced one, so a shared key is written in both shapes.
		if slices.Contains(sharedKeys, key) {
			transformerCfg[key] = value
		}
	}

	return providerCfg, transformerCfg
}

func isTransformerConfigKey(key string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// WriteBluelinkConfig writes a Bluelink deploy config to a file.
func WriteBluelinkConfig(path string, cfg *BluelinkDeployConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling bluelink deploy config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Removes single-line (//) and block comments from JSONC, along with trailing
// commas before } or ].
func stripJSONCComments(s string) string {
	// Remove block comments
	result := strings.Builder{}
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			// Skip block comment
			end := strings.Index(s[i+2:], "*/")
			if end == -1 {
				break
			}
			i += end + 4
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			// Skip line comment
			end := strings.IndexByte(s[i:], '\n')
			if end == -1 {
				break
			}
			i += end
			continue
		}
		if s[i] == '"' {
			// Skip string literals (don't strip comments inside strings)
			result.WriteByte(s[i])
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					result.WriteByte(s[i])
					i++
				}
				result.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				result.WriteByte(s[i])
				i++
			}
			continue
		}
		result.WriteByte(s[i])
		i++
	}

	cleaned := result.String()
	// Remove trailing commas before } or ] (with optional whitespace/newlines between)
	trailingCommaRe := regexp.MustCompile(`,(\s*[}\]])`)
	cleaned = trailingCommaRe.ReplaceAllString(cleaned, "$1")
	return cleaned
}

package build

import (
	"testing"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Deploy target settings are written namespaced in app.deploy.jsonc and reach
// the uploader in that form, so a bare-key lookup never matches. With the
// region unmatched it defaulted to us-east-1, artifacts were uploaded to a
// bucket in that region, and Lambda refused to read them from the region the
// blueprint deploys to.
func Test_aws_config_value_reads_the_namespaced_key(t *testing.T) {
	config := map[string]any{
		"aws.region":  "eu-west-2",
		"aws.profile": "some-profile",
	}

	assert.Equal(t, "eu-west-2", awsConfigValue(config, "region", "us-east-1"))
	assert.Equal(t, "some-profile", awsConfigValue(config, "profile", ""))
}

// A caller holding an already-stripped config keeps working.
func Test_aws_config_value_falls_back_to_the_bare_key(t *testing.T) {
	config := map[string]any{"region": "eu-west-1"}

	assert.Equal(t, "eu-west-1", awsConfigValue(config, "region", "us-east-1"))
}

func Test_aws_config_value_prefers_the_namespaced_key(t *testing.T) {
	config := map[string]any{
		"aws.region": "eu-west-2",
		"region":     "ap-southeast-1",
	}

	assert.Equal(t, "eu-west-2", awsConfigValue(config, "region", "us-east-1"))
}

func Test_aws_config_value_uses_the_default_when_absent(t *testing.T) {
	assert.Equal(t, "us-east-1", awsConfigValue(map[string]any{}, "region", "us-east-1"))
	assert.Equal(t, "us-east-1", awsConfigValue(nil, "region", "us-east-1"))
}

// Every client the uploader builds must resolve credentials the same way, or
// the account lookup that names the bucket resolves a different account from
// the one the artifacts are uploaded to.
func Test_uploader_applies_the_profile_to_its_aws_config(t *testing.T) {
	uploader := newS3Uploader(map[string]any{
		"aws.region":  "eu-west-2",
		"aws.profile": "some-profile",
	})

	loadOptions := awsconfig.LoadOptions{}
	for _, apply := range uploader.awsConfigOptions() {
		require.NoError(t, apply(&loadOptions))
	}

	assert.Equal(t, "some-profile", loadOptions.SharedConfigProfile)
	assert.Equal(t, "eu-west-2", loadOptions.Region)
}

func Test_uploader_omits_the_profile_when_none_is_configured(t *testing.T) {
	uploader := newS3Uploader(map[string]any{"aws.region": "eu-west-2"})

	loadOptions := awsconfig.LoadOptions{}
	for _, apply := range uploader.awsConfigOptions() {
		require.NoError(t, apply(&loadOptions))
	}

	assert.Empty(t, loadOptions.SharedConfigProfile)
}

// The bucket is per account and region, so a region that was never read sends
// artifacts somewhere the deployment cannot use them.
func Test_uploader_takes_its_region_from_the_deploy_target(t *testing.T) {
	uploader := newS3Uploader(map[string]any{"aws.region": "eu-west-2"})

	assert.Equal(t, "eu-west-2", uploader.region)
}

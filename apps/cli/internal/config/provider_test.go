package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/suite"
)

type ProviderTestSuite struct {
	suite.Suite
}

func TestProviderTestSuite(t *testing.T) {
	suite.Run(t, new(ProviderTestSuite))
}

func (s *ProviderTestSuite) newFlag(value string, changed bool) *pflag.Flag {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("test-flag", value, "")
	f := fs.Lookup("test-flag")
	f.Changed = changed
	return f
}

func (s *ProviderTestSuite) Test_get_string_flag_set_by_user_wins() {
	p := NewProvider()
	p.BindPFlag("key", s.newFlag("from-flag", true))
	p.BindEnvVar("key", "TEST_PROVIDER_ENV")
	s.T().Setenv("TEST_PROVIDER_ENV", "from-env")
	p.config["key"] = "from-file"

	val, isDefault := p.GetString("key")
	s.Assert().Equal("from-flag", val)
	s.Assert().False(isDefault)
}

func (s *ProviderTestSuite) Test_get_string_env_var_beats_config_file() {
	p := NewProvider()
	p.BindEnvVar("key", "TEST_PROVIDER_ENV2")
	s.T().Setenv("TEST_PROVIDER_ENV2", "from-env")
	p.config["key"] = "from-file"

	val, isDefault := p.GetString("key")
	s.Assert().Equal("from-env", val)
	s.Assert().False(isDefault)
}

func (s *ProviderTestSuite) Test_get_string_config_file_beats_flag_default() {
	p := NewProvider()
	p.BindPFlag("key", s.newFlag("flag-default", false))
	p.config["key"] = "from-file"

	val, isDefault := p.GetString("key")
	s.Assert().Equal("from-file", val)
	s.Assert().False(isDefault)
}

func (s *ProviderTestSuite) Test_get_string_flag_default_beats_provider_default() {
	p := NewProvider()
	p.BindPFlag("key", s.newFlag("flag-default", false))
	p.SetDefault("key", "provider-default")

	val, isDefault := p.GetString("key")
	s.Assert().Equal("flag-default", val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_string_provider_default_as_last_resort() {
	p := NewProvider()
	p.SetDefault("key", "provider-default")

	val, isDefault := p.GetString("key")
	s.Assert().Equal("provider-default", val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_string_empty_when_nothing_set() {
	p := NewProvider()
	val, isDefault := p.GetString("missing")
	s.Assert().Equal("", val)
	s.Assert().True(isDefault)
}

// --- Typed getters ---

func (s *ProviderTestSuite) Test_get_int32_valid() {
	p := NewProvider()
	p.config["port"] = "8080"
	val, isDefault := p.GetInt32("port")
	s.Assert().Equal(int32(8080), val)
	s.Assert().False(isDefault)
}

func (s *ProviderTestSuite) Test_get_int32_invalid_returns_zero() {
	p := NewProvider()
	p.config["port"] = "not-a-number"
	val, _ := p.GetInt32("port")
	s.Assert().Equal(int32(0), val)
}

func (s *ProviderTestSuite) Test_get_int64_valid() {
	p := NewProvider()
	p.config["big"] = "9999999999"
	val, _ := p.GetInt64("big")
	s.Assert().Equal(int64(9999999999), val)
}

func (s *ProviderTestSuite) Test_get_uint32_valid() {
	p := NewProvider()
	p.config["count"] = "42"
	val, _ := p.GetUint32("count")
	s.Assert().Equal(uint32(42), val)
}

func (s *ProviderTestSuite) Test_get_uint64_valid() {
	p := NewProvider()
	p.config["count"] = "18446744073709551615"
	val, _ := p.GetUint64("count")
	s.Assert().Equal(uint64(18446744073709551615), val)
}

func (s *ProviderTestSuite) Test_get_float32_valid() {
	p := NewProvider()
	p.config["ratio"] = "3.14"
	val, _ := p.GetFloat32("ratio")
	s.Assert().InDelta(float32(3.14), val, 0.01)
}

func (s *ProviderTestSuite) Test_get_float64_valid() {
	p := NewProvider()
	p.config["ratio"] = "3.14159265358979"
	val, _ := p.GetFloat64("ratio")
	s.Assert().InDelta(3.14159265358979, val, 0.0001)
}

func (s *ProviderTestSuite) Test_get_bool_true() {
	p := NewProvider()
	p.config["flag"] = "true"
	val, _ := p.GetBool("flag")
	s.Assert().True(val)
}

func (s *ProviderTestSuite) Test_get_bool_false() {
	p := NewProvider()
	p.config["flag"] = "false"
	val, _ := p.GetBool("flag")
	s.Assert().False(val)
}

func (s *ProviderTestSuite) Test_get_bool_invalid_returns_false() {
	p := NewProvider()
	p.config["flag"] = "maybe"
	val, _ := p.GetBool("flag")
	s.Assert().False(val)
}

func (s *ProviderTestSuite) Test_get_bool_empty_returns_false() {
	p := NewProvider()
	val, isDefault := p.GetBool("missing")
	s.Assert().False(val)
	s.Assert().True(isDefault)
}

// --- LoadConfigFile ---

func (s *ProviderTestSuite) Test_load_yaml_config() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte("app_port: \"9090\"\napp_name: myapp\n"), 0o644)
	s.Require().NoError(err)

	p := NewProvider()
	err = p.LoadConfigFile(path)
	s.Require().NoError(err)

	val, _ := p.GetString("app_port")
	s.Assert().Equal("9090", val)
	val, _ = p.GetString("app_name")
	s.Assert().Equal("myapp", val)
}

func (s *ProviderTestSuite) Test_load_json_config() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte(`{"debug": "true", "port": "3000"}`), 0o644)
	s.Require().NoError(err)

	p := NewProvider()
	err = p.LoadConfigFile(path)
	s.Require().NoError(err)

	val, _ := p.GetString("port")
	s.Assert().Equal("3000", val)
}

func (s *ProviderTestSuite) Test_load_toml_config() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "config.toml")
	err := os.WriteFile(path, []byte("host = \"localhost\"\nport = \"5432\"\n"), 0o644)
	s.Require().NoError(err)

	p := NewProvider()
	err = p.LoadConfigFile(path)
	s.Require().NoError(err)

	val, _ := p.GetString("host")
	s.Assert().Equal("localhost", val)
}

func (s *ProviderTestSuite) Test_load_unsupported_format_returns_error() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "config.xml")
	err := os.WriteFile(path, []byte("<config/>"), 0o644)
	s.Require().NoError(err)

	p := NewProvider()
	err = p.LoadConfigFile(path)
	s.Assert().ErrorIs(err, ErrUnsupportedConfigFileFormat)
}

func (s *ProviderTestSuite) Test_load_nonexistent_file_returns_error() {
	p := NewProvider()
	err := p.LoadConfigFile("/nonexistent/path/config.yaml")
	s.Assert().Error(err)
}

func (s *ProviderTestSuite) Test_load_yml_extension() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte("key: value\n"), 0o644)
	s.Require().NoError(err)

	p := NewProvider()
	s.Require().NoError(p.LoadConfigFile(path))
	val, _ := p.GetString("key")
	s.Assert().Equal("value", val)
}

func (s *ProviderTestSuite) Test_get_int32_empty_returns_zero() {
	p := NewProvider()
	val, isDefault := p.GetInt32("missing")
	s.Assert().Equal(int32(0), val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_int64_empty_returns_zero() {
	p := NewProvider()
	val, isDefault := p.GetInt64("missing")
	s.Assert().Equal(int64(0), val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_int64_invalid_returns_zero() {
	p := NewProvider()
	p.config["x"] = "not-a-number"
	val, _ := p.GetInt64("x")
	s.Assert().Equal(int64(0), val)
}

func (s *ProviderTestSuite) Test_get_uint32_empty_returns_zero() {
	p := NewProvider()
	val, isDefault := p.GetUint32("missing")
	s.Assert().Equal(uint32(0), val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_uint32_invalid_returns_zero() {
	p := NewProvider()
	p.config["x"] = "abc"
	val, _ := p.GetUint32("x")
	s.Assert().Equal(uint32(0), val)
}

func (s *ProviderTestSuite) Test_get_uint64_empty_returns_zero() {
	p := NewProvider()
	val, isDefault := p.GetUint64("missing")
	s.Assert().Equal(uint64(0), val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_uint64_invalid_returns_zero() {
	p := NewProvider()
	p.config["x"] = "abc"
	val, _ := p.GetUint64("x")
	s.Assert().Equal(uint64(0), val)
}

func (s *ProviderTestSuite) Test_get_float32_empty_returns_zero() {
	p := NewProvider()
	val, isDefault := p.GetFloat32("missing")
	s.Assert().Equal(float32(0), val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_float32_invalid_returns_zero() {
	p := NewProvider()
	p.config["x"] = "abc"
	val, _ := p.GetFloat32("x")
	s.Assert().Equal(float32(0), val)
}

func (s *ProviderTestSuite) Test_get_float64_empty_returns_zero() {
	p := NewProvider()
	val, isDefault := p.GetFloat64("missing")
	s.Assert().Equal(float64(0), val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_float64_invalid_returns_zero() {
	p := NewProvider()
	p.config["x"] = "abc"
	val, _ := p.GetFloat64("x")
	s.Assert().Equal(float64(0), val)
}

func (s *ProviderTestSuite) Test_get_string_whitespace_env_var_ignored() {
	p := NewProvider()
	p.BindEnvVar("key", "TEST_PROVIDER_WHITESPACE")
	s.T().Setenv("TEST_PROVIDER_WHITESPACE", "   ")
	p.SetDefault("key", "default")

	val, isDefault := p.GetString("key")
	s.Assert().Equal("default", val)
	s.Assert().True(isDefault)
}

func (s *ProviderTestSuite) Test_get_string_whitespace_flag_falls_through() {
	p := NewProvider()
	p.BindPFlag("key", s.newFlag("  ", false))
	p.SetDefault("key", "default")

	val, isDefault := p.GetString("key")
	s.Assert().Equal("default", val)
	s.Assert().True(isDefault)
}

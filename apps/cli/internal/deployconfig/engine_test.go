package deployconfig

import (
	"testing"

	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/stretchr/testify/suite"
)

type EngineTestSuite struct {
	suite.Suite
}

func TestEngineTestSuite(t *testing.T) {
	suite.Run(t, new(EngineTestSuite))
}

func (s *EngineTestSuite) newProvider(protocol, endpoint string) *config.Provider {
	p := config.NewProvider()
	p.BindEnvVar("connectProtocol", "CELERITY_CLI_TEST_CONNECT_PROTOCOL")
	p.BindEnvVar("engineEndpoint", "CELERITY_CLI_TEST_ENGINE_ENDPOINT")
	s.T().Setenv("CELERITY_CLI_TEST_CONNECT_PROTOCOL", protocol)
	s.T().Setenv("CELERITY_CLI_TEST_ENGINE_ENDPOINT", endpoint)
	return p
}

func (s *EngineTestSuite) Test_IsRemoteEngine_unixProtocol_alwaysFalse() {
	cases := []string{
		"",
		"http://engine.example.com:8325",
		"http://localhost:8325",
	}
	for _, endpoint := range cases {
		p := s.newProvider("unix", endpoint)
		s.False(IsRemoteEngine(p), "unix protocol with endpoint=%q", endpoint)
	}
}

func (s *EngineTestSuite) Test_IsRemoteEngine_tcp_localhost_isLocal() {
	cases := []string{
		"http://localhost:8325",
		"http://127.0.0.1:8325",
		"http://[::1]:8325",
		"https://localhost",
	}
	for _, endpoint := range cases {
		p := s.newProvider("tcp", endpoint)
		s.False(IsRemoteEngine(p), "loopback endpoint=%q should be local", endpoint)
	}
}

func (s *EngineTestSuite) Test_IsRemoteEngine_tcp_remoteHost_isRemote() {
	cases := []string{
		"http://engine.example.com:8325",
		"http://10.0.0.5:8325",
		"https://celerity.internal",
	}
	for _, endpoint := range cases {
		p := s.newProvider("tcp", endpoint)
		s.True(IsRemoteEngine(p), "non-loopback endpoint=%q should be remote", endpoint)
	}
}

func (s *EngineTestSuite) Test_IsRemoteEngine_tcp_emptyEndpoint_isLocal() {
	p := s.newProvider("tcp", "")
	s.False(IsRemoteEngine(p))
}

func (s *EngineTestSuite) Test_IsRemoteEngine_tcp_unparseableEndpoint_isLocal() {
	p := s.newProvider("tcp", "://bad-url")
	s.False(IsRemoteEngine(p))
}

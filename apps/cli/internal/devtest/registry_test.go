package devtest

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type RegistryTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}

func (s *RegistryTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *RegistryTestSuite) Test_nodejs_runtime_returns_node_runner() {
	runner, err := RunnerForRuntime("nodejs22.x", s.logger)
	s.Require().NoError(err)
	s.Assert().IsType(&NodeRunner{}, runner)
}

func (s *RegistryTestSuite) Test_python_runtime_returns_python_runner() {
	runner, err := RunnerForRuntime("python3.12.x", s.logger)
	s.Require().NoError(err)
	s.Assert().IsType(&PythonRunner{}, runner)
}

func (s *RegistryTestSuite) Test_unknown_runtime_returns_error() {
	_, err := RunnerForRuntime("rust1.75", s.logger)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "no test runner for runtime")
}

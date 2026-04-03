package devtest

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type InfraTestSuite struct {
	suite.Suite
}

func TestInfraTestSuite(t *testing.T) {
	suite.Run(t, new(InfraTestSuite))
}

func (s *InfraTestSuite) Test_unit_only_returns_none() {
	level := InfraLevelForSuites([]TestSuite{SuiteUnit})
	s.Assert().Equal(InfraLevelNone, level)
}

func (s *InfraTestSuite) Test_integration_returns_compose() {
	level := InfraLevelForSuites([]TestSuite{SuiteIntegration})
	s.Assert().Equal(InfraLevelCompose, level)
}

func (s *InfraTestSuite) Test_api_returns_full() {
	level := InfraLevelForSuites([]TestSuite{SuiteAPI})
	s.Assert().Equal(InfraLevelFull, level)
}

func (s *InfraTestSuite) Test_mixed_unit_and_integration_returns_compose() {
	level := InfraLevelForSuites([]TestSuite{SuiteUnit, SuiteIntegration})
	s.Assert().Equal(InfraLevelCompose, level)
}

func (s *InfraTestSuite) Test_mixed_unit_and_api_returns_full() {
	level := InfraLevelForSuites([]TestSuite{SuiteUnit, SuiteAPI})
	s.Assert().Equal(InfraLevelFull, level)
}

func (s *InfraTestSuite) Test_mixed_integration_and_api_returns_full() {
	level := InfraLevelForSuites([]TestSuite{SuiteIntegration, SuiteAPI})
	s.Assert().Equal(InfraLevelFull, level)
}

func (s *InfraTestSuite) Test_empty_suites_returns_none() {
	level := InfraLevelForSuites(nil)
	s.Assert().Equal(InfraLevelNone, level)
}

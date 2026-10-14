package sqlschema

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// enableMySQLForTest turns the gated MySQL dialect on for the duration of a test,
// restoring the previous state afterwards.
func enableMySQLForTest(t *testing.T) {
	prev := mysqlEnabled
	mysqlEnabled = true
	t.Cleanup(func() { mysqlEnabled = prev })
}

type DialectGateSuite struct {
	suite.Suite
}

func TestDialectGateSuite(t *testing.T) {
	suite.Run(t, new(DialectGateSuite))
}

func (s *DialectGateSuite) Test_mysql_gated_off_by_default() {
	_, err := dialectForEngine("mysql")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "not yet supported")
}

func (s *DialectGateSuite) Test_mysql_available_when_enabled() {
	enableMySQLForTest(s.T())
	d, err := dialectForEngine("mysql")
	s.Require().NoError(err)
	s.Assert().Equal("mysql", d.DriverName())
}

func (s *DialectGateSuite) Test_postgres_always_available() {
	d, err := dialectForEngine("postgres")
	s.Require().NoError(err)
	s.Assert().Equal("pgx", d.DriverName())
}

func (s *DialectGateSuite) Test_unknown_engine_errors() {
	_, err := dialectForEngine("oracle")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "unsupported SQL engine")
}

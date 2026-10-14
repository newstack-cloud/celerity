package sqlschema

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// The application role is the one piece of this that cannot be verified by
// asserting on generated SQL: whether a grant actually lets a connection read a
// table is a question only a database answers.
type ProvisionIntegrationSuite struct {
	suite.Suite
	connStr string
	logger  *zap.Logger
}

func TestProvisionIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ProvisionIntegrationSuite))
}

const testAppUser = "celerity_app_provision_test"

func (s *ProvisionIntegrationSuite) SetupTest() {
	host := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_HOST")
	port := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PORT")
	user := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_USER")
	password := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PASSWORD")
	database := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_DATABASE")

	s.connStr = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, database)
	s.logger = zap.NewNop()
	s.cleanup()
}

func (s *ProvisionIntegrationSuite) TearDownTest() {
	s.cleanup()
}

// Dropping a role means undoing everything granted to it first, including the
// default privileges, which are not owned by the role and so do not go with it.
func (s *ProvisionIntegrationSuite) cleanup() {
	db, err := sql.Open("pgx", s.connStr)
	if err != nil {
		return
	}
	defer db.Close()

	_, _ = db.Exec("DROP TABLE IF EXISTS provision_test_orders CASCADE")
	_, _ = db.Exec(fmt.Sprintf(
		`ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM %q`, testAppUser))
	_, _ = db.Exec(fmt.Sprintf(
		`ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM %q`, testAppUser))
	_, _ = db.Exec(fmt.Sprintf(`DROP OWNED BY %q CASCADE`, testAppUser))
	_, _ = db.Exec(fmt.Sprintf(`DROP ROLE IF EXISTS %q`, testAppUser))
}

func (s *ProvisionIntegrationSuite) provision(password string) *ProvisionReport {
	applier, err := NewApplier("postgres", s.connStr, s.logger)
	s.Require().NoError(err)
	defer applier.Close()

	report, err := applier.ProvisionAppUser(context.Background(), AppUser{
		Name:     testAppUser,
		Password: password,
	})
	s.Require().NoError(err)
	return report
}

// appConn opens a connection as the provisioned application user, which is the
// only way to be sure the grants mean what they are supposed to.
func (s *ProvisionIntegrationSuite) appConn(password string) *sql.DB {
	host := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_HOST")
	port := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_PORT")
	database := testutils.RequireEnv(s.T(), "CELERITY_TEST_POSTGRES_DATABASE")

	// url.URL rather than a formatted string: a password is arbitrary text and
	// has to be percent-encoded to survive a connection URL.
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(testAppUser, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}
	db, err := sql.Open("pgx", u.String())
	s.Require().NoError(err)
	s.Require().NoError(db.Ping(), "the provisioned user could not authenticate")
	return db
}

func (s *ProvisionIntegrationSuite) ownerExec(statements ...string) {
	db, err := sql.Open("pgx", s.connStr)
	s.Require().NoError(err)
	defer db.Close()
	for _, stmt := range statements {
		_, err := db.Exec(stmt)
		s.Require().NoError(err, stmt)
	}
}

func (s *ProvisionIntegrationSuite) Test_provisioned_user_can_authenticate_and_read_and_write() {
	const password = "provision-test-password"
	s.ownerExec(`CREATE TABLE provision_test_orders (id serial PRIMARY KEY, label text)`)

	report := s.provision(password)
	s.True(report.RoleCreated)
	s.True(report.AuthConfigured)

	app := s.appConn(password)
	defer app.Close()

	// The sequence grant matters as much as the table grant: without it an
	// insert into a serial column fails on permission for the sequence, which
	// reads as a mystifying error at runtime.
	_, err := app.Exec(`INSERT INTO provision_test_orders (label) VALUES ('first')`)
	s.Require().NoError(err)

	var label string
	s.Require().NoError(
		app.QueryRow(`SELECT label FROM provision_test_orders`).Scan(&label))
	s.Equal("first", label)
}

// The reason default privileges are part of provisioning: without them the
// application cannot see anything a later migration creates, and provisioning
// would have to run again after every migration.
func (s *ProvisionIntegrationSuite) Test_tables_created_after_provisioning_are_usable() {
	const password = "provision-test-password"
	s.provision(password)

	s.ownerExec(`CREATE TABLE provision_test_orders (id serial PRIMARY KEY, label text)`)

	app := s.appConn(password)
	defer app.Close()

	_, err := app.Exec(`INSERT INTO provision_test_orders (label) VALUES ('later')`)
	s.Require().NoError(err, "a table created after provisioning must be writable without re-provisioning")
}

// The application gets DML and nothing structural. Migrations are the only
// thing that changes shape, and they run as the owner.
func (s *ProvisionIntegrationSuite) Test_application_user_cannot_change_the_schema() {
	const password = "provision-test-password"
	s.provision(password)

	app := s.appConn(password)
	defer app.Close()

	_, err := app.Exec(`CREATE TABLE provision_test_forbidden (id int)`)
	s.Require().Error(err, "the application user must not be able to create tables")
	s.Contains(err.Error(), "permission denied")
}

// Provisioning runs after every deploy, so a second run against a database that
// already has the role must be a no-op that reports as much.
func (s *ProvisionIntegrationSuite) Test_provisioning_twice_reports_no_change() {
	const password = "provision-test-password"
	s.provision(password)

	report := s.provision(password)

	s.False(report.RoleCreated, "the role already existed")
	s.NotEmpty(report.Grants, "grants are re-applied regardless")
}

// Rotating the secret has to reach the database, and the only thing that
// carries it there is a repeat provisioning run.
func (s *ProvisionIntegrationSuite) Test_a_changed_password_is_applied() {
	s.provision("first-password")

	s.provision("second-password")

	app := s.appConn("second-password")
	defer app.Close()
}

// Generated passwords exclude quotes, but a database restored from elsewhere or
// a hand-set secret need not, and a password mangled by quoting produces a
// login failure with no clue as to why.
func (s *ProvisionIntegrationSuite) Test_a_password_with_quotes_and_backslashes_round_trips() {
	const awkward = `p'a"s\s w%o$rd`

	s.provision(awkward)

	app := s.appConn(awkward)
	defer app.Close()

	var one int
	s.Require().NoError(app.QueryRow("SELECT 1").Scan(&one))
}

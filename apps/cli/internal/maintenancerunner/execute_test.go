package maintenancerunner

import (
	"context"
	"errors"
	"os"
	"sort"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn"
	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeApplier struct {
	provisioned sqlschema.AppUser
	report      *sqlschema.ProvisionReport
	err         error
	closed      bool

	// Migration state: what the runner was asked to apply, and what the
	// database claims is pending.
	pending      []sqlschema.PendingMigration
	pendingErr   error
	appliedDir   string
	appliedRisk  sqlschema.RiskLevel
	applyErr     error
	appliedFiles []string
}

func (f *fakeApplier) ProvisionAppUser(_ context.Context, user sqlschema.AppUser) (*sqlschema.ProvisionReport, error) {
	f.provisioned = user
	if f.err != nil {
		return nil, f.err
	}
	if f.report != nil {
		return f.report, nil
	}
	return &sqlschema.ProvisionReport{User: user.Name, RoleCreated: true, AuthConfigured: true}, nil
}

func (f *fakeApplier) PendingMigrations(
	_ context.Context,
	migrationsPath string,
) ([]sqlschema.PendingMigration, error) {
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	// Record what the runner actually materialised, so a test can assert the
	// files arrived rather than only that a directory was passed.
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		f.appliedFiles = append(f.appliedFiles, entry.Name())
	}
	sort.Strings(f.appliedFiles)
	return f.pending, nil
}

func (f *fakeApplier) ApplyMigrationDir(
	_ context.Context,
	migrationsPath string,
	maxRisk sqlschema.RiskLevel,
) error {
	f.appliedDir = migrationsPath
	f.appliedRisk = maxRisk
	return f.applyErr
}

func (f *fakeApplier) Close() error {
	f.closed = true
	return nil
}

// fakeCredentials stands in for a provider's secret store. Everything the
// executor does with a credential is visible through it: which one it uses to
// connect, and which it only writes into the database.
type fakeCredentials struct {
	stored map[string]dbconn.Credentials
}

func (f *fakeCredentials) Resolve(_ context.Context, reference string) (dbconn.Credentials, error) {
	creds, ok := f.stored[reference]
	if !ok {
		return dbconn.Credentials{}, errors.New("AccessDeniedException")
	}
	return creds, nil
}

func (f *fakeCredentials) Token(
	_ context.Context, _ dbconn.Endpoint, user string, _ string,
) (dbconn.Credentials, error) {
	return dbconn.Credentials{Username: user, Password: "issued-token"}, nil
}

type harness struct {
	executor *Executor
	applier  *fakeApplier
	// connStr is what the executor would have connected with, which is where
	// the credential and the TLS mode end up.
	connStr string
	secrets *fakeCredentials
}

func newHarness(targets Targets) *harness {
	h := &harness{
		applier: &fakeApplier{},
		secrets: &fakeCredentials{stored: map[string]dbconn.Credentials{}},
	}
	h.executor = NewExecutor(targets, h.secrets, zap.NewNop())
	h.executor.connect = func(_ context.Context, _ string, connStr string) (applier, error) {
		h.connStr = connStr
		return h.applier, nil
	}
	return h
}

// store records a credential the provider's secret store holds.
func (h *harness) store(reference, username, password string) {
	h.secrets.stored[reference] = dbconn.Credentials{Username: username, Password: password}
}

func passwordTarget() Targets {
	return Targets{"ordersDb": {
		Engine:               "postgres",
		Host:                 "orders.proxy-abc.eu-west-2.rds.amazonaws.com",
		Port:                 "5432",
		Database:             "orders",
		OwnerCredentialRef:   "arn:aws:secretsmanager:eu-west-2:1:secret:orders-master",
		AppUser:              "celerity_app",
		AppUserCredentialRef: "arn:aws:secretsmanager:eu-west-2:1:secret:orders-app-user",
	}}
}

func TestProvisionsTheApplicationUserFromItsOwnSecret(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "celerity_master", "master-password")
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-app-user", "celerity_app", "app-password")

	resp := h.executor.Handle(context.Background(), Request{
		Operation: OpProvisionUser, Database: "ordersDb",
	})

	require.True(t, resp.OK, resp.Error)
	assert.Equal(t, "celerity_app", h.applier.provisioned.Name)
	assert.Equal(t, "app-password", h.applier.provisioned.Password)
	assert.True(t, resp.Provision.RoleCreated)
	assert.True(t, h.applier.closed, "the connection must not be left open")
}

// The whole point of the design: the runner connects as the owner, and the
// application's credential is only ever written into the database, never used
// to connect with.
func TestConnectsAsTheOwnerNotTheApplicationUser(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "celerity_master", "master-password")
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-app-user", "celerity_app", "app-password")

	h.executor.Handle(context.Background(), Request{Operation: OpProvisionUser, Database: "ordersDb"})

	assert.Contains(t, h.connStr, "celerity_master")
	assert.NotContains(t, h.connStr, "app-password")
	// A managed database is always reached over TLS.
	assert.Contains(t, h.connStr, "sslmode=require")
}

// IAM mode distributes no credential at all, so there is no application secret
// to read. The role still has to exist, which is why provisioning runs in both
// modes.
func TestIAMModeNeedsNoApplicationSecret(t *testing.T) {
	targets := passwordTarget()
	targets["ordersDb"].AppUserCredentialRef = ""
	targets["ordersDb"].AppUserTokenAuth = true
	h := newHarness(targets)
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "celerity_master", "master-password")

	resp := h.executor.Handle(context.Background(), Request{
		Operation: OpProvisionUser, Database: "ordersDb",
	})

	require.True(t, resp.OK, resp.Error)
	assert.True(t, h.applier.provisioned.IAMAuth)
	assert.Empty(t, h.applier.provisioned.Password)
}

// A database whose master is itself IAM-authenticated has no master secret to
// read; the runner exchanges its execution role instead.
func TestOwnerCanAuthenticateByIAM(t *testing.T) {
	targets := passwordTarget()
	targets["ordersDb"].OwnerCredentialRef = ""
	targets["ordersDb"].OwnerUser = "celerity_master"
	targets["ordersDb"].Region = "eu-west-2"
	h := newHarness(targets)
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-app-user", "celerity_app", "app-password")

	resp := h.executor.Handle(context.Background(), Request{
		Operation: OpProvisionUser, Database: "ordersDb",
	})

	require.True(t, resp.OK, resp.Error)
	assert.Contains(t, h.connStr, "issued-token")
}

// An operation that fails is a successful invocation carrying the reason. A
// Lambda error would reach the operator as a stack trace with the actual cause
// buried in CloudWatch.
func TestAFailedOperationIsReportedNotThrown(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "celerity_master", "master-password")
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-app-user", "celerity_app", "app-password")
	h.applier.err = errors.New("permission denied for schema public")

	resp := h.executor.Handle(context.Background(), Request{
		Operation: OpProvisionUser, Database: "ordersDb",
	})

	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "permission denied for schema public")
	assert.Equal(t, "ordersDb", resp.Database)
}

// The most likely real failure is the runner's role not being granted the
// secret, where the raw error reports only AccessDenied.
func TestAnUnreadableOwnerSecretSaysWhichCredentialFailed(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-app-user", "celerity_app", "app-password")

	resp := h.executor.Handle(context.Background(), Request{
		Operation: OpProvisionUser, Database: "ordersDb",
	})

	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "database owner's credentials")
	assert.Contains(t, resp.Error, "AccessDeniedException")
}

// A name the runner does not serve is far more often a stale deploy than a
// typo, so the error lists the databases this runner is configured for.
func TestAnUnknownDatabaseListsWhatTheRunnerServes(t *testing.T) {
	h := newHarness(passwordTarget())

	resp := h.executor.Handle(context.Background(), Request{
		Operation: OpProvisionUser, Database: "auditDb",
	})

	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, `no database named "auditDb"`)
	assert.Contains(t, resp.Error, "ordersDb")
}

func TestAnUnknownOperationIsRefused(t *testing.T) {
	h := newHarness(passwordTarget())

	resp := h.executor.Handle(context.Background(), Request{
		Operation: "execute-sql", Database: "ordersDb",
	})

	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, `unknown operation "execute-sql"`)
}

// Ping exists so a deploy can prove the network path works without changing
// anything in the database.
func TestPingConnectsAndChangesNothing(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "celerity_master", "master-password")

	resp := h.executor.Handle(context.Background(), Request{Operation: OpPing, Database: "ordersDb"})

	require.True(t, resp.OK, resp.Error)
	assert.Empty(t, h.applier.provisioned.Name, "ping must not provision anything")
	assert.True(t, h.applier.closed)
}

func TestTargetsRoundTripThroughTheEnvironmentFormat(t *testing.T) {
	targets, err := ParseTargets(`{"ordersDb":{"engine":"postgres","host":"h","port":"5432",` +
		`"database":"orders","ownerCredentialRef":"a","appUser":"celerity_app",` +
		`"appUserCredentialRef":"b"}}`)
	require.NoError(t, err)

	target, err := targets.Lookup("ordersDb")
	require.NoError(t, err)
	assert.Equal(t, "celerity_app", target.AppUser)
	assert.Equal(t, "a", target.OwnerCredentialRef)
	assert.Equal(t, "b", target.AppUserCredentialRef)
}

func TestARunnerWithNoTargetsSaysSo(t *testing.T) {
	_, err := Targets{}.Lookup("ordersDb")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no database targets configured")
}

// The whole point of the operation: migration files live on the operator's
// machine and the database is unreachable from it, so the files travel and the
// runner applies them locally.
func TestApplyWritesTheSentMigrationsAndRunsThemAsTheOwner(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "postgres", "master-pw")
	h.applier.pending = []sqlschema.PendingMigration{
		{Name: "20260101000000_initial.sql", Risk: sqlschema.RiskSafe},
	}

	resp := h.executor.Handle(context.Background(), Request{
		Operation: OpApply,
		Database:  "ordersDb",
		Migrations: []MigrationFile{
			{Name: "20260101000000_initial.sql", Content: "CREATE TABLE orders (id int);"},
			{Name: "atlas.sum", Content: "h1:abc\n"},
		},
	})

	require.True(t, resp.OK, "expected apply to succeed, got: %s", resp.Error)
	assert.Equal(t, []string{"20260101000000_initial.sql"}, resp.Apply.Applied)
	assert.Equal(t, []string{"20260101000000_initial.sql", "atlas.sum"}, h.applier.appliedFiles,
		"the runner must materialise every file it was sent, including the checksum")
	assert.Contains(t, h.connStr, "master-pw", "migrations run as the database owner")
	assert.True(t, h.applier.closed)
}

// The risk gate is the operator's authorisation, so it has to survive the trip
// rather than being re-decided by the side that cannot see the flags.
func TestApplyHoldsBackMigrationsAboveTheAuthorisedRisk(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "postgres", "master-pw")
	h.applier.pending = []sqlschema.PendingMigration{
		{Name: "001_safe.sql", Risk: sqlschema.RiskSafe},
		{Name: "002_risky.sql", Risk: sqlschema.RiskRisky},
		{Name: "003_breaking.sql", Risk: sqlschema.RiskBreaking},
	}

	resp := h.executor.Handle(context.Background(), Request{
		Operation:  OpApply,
		Database:   "ordersDb",
		MaxRisk:    string(sqlschema.RiskRisky),
		Migrations: []MigrationFile{{Name: "001_safe.sql", Content: "SELECT 1;"}},
	})

	require.True(t, resp.OK, resp.Error)
	assert.Equal(t, []string{"001_safe.sql", "002_risky.sql"}, resp.Apply.Applied)
	require.Len(t, resp.Apply.Blocked, 1)
	assert.Equal(t, "003_breaking.sql", resp.Apply.Blocked[0].Name)
	assert.Equal(t, sqlschema.RiskRisky, h.applier.appliedRisk)
}

// An absent MaxRisk must mean "safe only" rather than "no limit". A request that
// lost the field in transit should refuse to run a destructive migration rather
// than authorise every one of them.
func TestApplyWithoutAnAuthorisedRiskAppliesOnlySafeMigrations(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "postgres", "master-pw")
	h.applier.pending = []sqlschema.PendingMigration{
		{Name: "001_breaking.sql", Risk: sqlschema.RiskBreaking},
	}

	resp := h.executor.Handle(context.Background(), Request{
		Operation:  OpApply,
		Database:   "ordersDb",
		Migrations: []MigrationFile{{Name: "001_breaking.sql", Content: "DROP TABLE orders;"}},
	})

	require.True(t, resp.OK, resp.Error)
	assert.Empty(t, resp.Apply.Applied)
	require.Len(t, resp.Apply.Blocked, 1)
	assert.Equal(t, sqlschema.RiskSafe, h.applier.appliedRisk)
}

// A file name that is a path could write outside the directory the runner
// created for it, so it is refused rather than cleaned up.
func TestApplyRefusesAMigrationNameThatIsAPath(t *testing.T) {
	h := newHarness(passwordTarget())
	h.store("arn:aws:secretsmanager:eu-west-2:1:secret:orders-master", "postgres", "master-pw")

	resp := h.executor.Handle(context.Background(), Request{
		Operation:  OpApply,
		Database:   "ordersDb",
		Migrations: []MigrationFile{{Name: "../../etc/passwd", Content: "nope"}},
	})

	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "bare file name")
}

func TestApplyWithNoMigrationsSaysSoRatherThanConnecting(t *testing.T) {
	h := newHarness(passwordTarget())

	resp := h.executor.Handle(context.Background(), Request{
		Operation: OpApply,
		Database:  "ordersDb",
	})

	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "no migration files")
	assert.Empty(t, h.connStr, "there is nothing to do, so nothing should have been connected to")
}

func TestAnUnknownOperationNamesTheOnesItSupports(t *testing.T) {
	h := newHarness(passwordTarget())

	resp := h.executor.Handle(context.Background(), Request{
		Operation: "drop-everything",
		Database:  "ordersDb",
	})

	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, OpApply)
	assert.Contains(t, resp.Error, OpProvisionUser)
}

package maintenancerunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn"
	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"go.uber.org/zap"
)

// Executor performs operations against the databases it was deployed to serve.
//
// Nothing in here is cloud-specific. It reads its targets, resolves credentials
// through a dbconn.CredentialSource and runs the same internal/sqlschema code
// the CLI runs locally. What differs per provider is how the executor is hosted
// (a Lambda on AWS) and where credentials come from, and both of those are
// supplied from outside.
//
// It holds no state between invocations beyond its targets, because a
// maintenance operation is rare enough that pooling a connection would only mean
// holding an idle one open against the database's connection limit.
type Executor struct {
	targets     Targets
	credentials dbconn.CredentialSource
	logger      *zap.Logger

	// connect is the seam the tests replace. Everything else here is dispatch,
	// validation and error reporting, none of which needs a real database.
	connect func(ctx context.Context, engine string, connStr string) (applier, error)
}

// applier is the part of sqlschema.Applier the executor uses.
type applier interface {
	ProvisionAppUser(ctx context.Context, user sqlschema.AppUser) (*sqlschema.ProvisionReport, error)
	PendingMigrations(ctx context.Context, migrationsPath string) ([]sqlschema.PendingMigration, error)
	ApplyMigrationDir(ctx context.Context, migrationsPath string, maxRisk sqlschema.RiskLevel) error
	Close() error
}

// NewExecutor builds an executor over a set of database targets, resolving
// credentials through the given provider's source.
func NewExecutor(targets Targets, credentials dbconn.CredentialSource, logger *zap.Logger) *Executor {
	return &Executor{
		targets:     targets,
		credentials: credentials,
		logger:      logger,
		connect:     connectToDatabase,
	}
}

// Handle runs one request and always returns a response. An operation that fails
// is reported in the response rather than as a host-level error, so the CLI has
// something to print beyond a stack trace.
func (e *Executor) Handle(ctx context.Context, req Request) *Response {
	e.logger.Info("maintenance request",
		zap.String("operation", req.Operation),
		zap.String("database", req.Database),
	)

	target, err := e.targets.Lookup(req.Database)
	if err != nil {
		return req.Failed(err)
	}

	switch req.Operation {
	case OpProvisionUser:
		return e.provisionUser(ctx, req, target)
	case OpPing:
		return e.ping(ctx, req, target)
	case OpApply:
		return e.applyMigrations(ctx, req, target)
	default:
		return req.Failed(fmt.Errorf(
			"unknown operation %q (this runner supports: %s, %s, %s)",
			req.Operation, OpProvisionUser, OpPing, OpApply))
	}
}

func (e *Executor) provisionUser(ctx context.Context, req Request, target *Target) *Response {
	appUser, err := e.appUser(ctx, target)
	if err != nil {
		return req.Failed(err)
	}

	db, err := e.connectAsOwner(ctx, target)
	if err != nil {
		return req.Failed(err)
	}
	defer db.Close()

	report, err := db.ProvisionAppUser(ctx, appUser)
	if err != nil {
		return req.Failed(err)
	}

	e.logger.Info("provisioned application user",
		zap.String("database", req.Database),
		zap.String("user", report.User),
		zap.Bool("roleCreated", report.RoleCreated),
	)
	return &Response{Operation: req.Operation, Database: req.Database, OK: true, Provision: report}
}

// Runs the migration set the request carried, as the database owner.
//
// The files are materialised into a directory of the runner's own rather than
// executed as statements, because everything downstream is defined over a
// migration directory: the checksum check, revision tracking and risk
// classification. Running the same sqlschema code over the same shape of input
// is what keeps the runner's behaviour identical to the local path rather than
// merely similar.
func (e *Executor) applyMigrations(ctx context.Context, req Request, target *Target) *Response {
	if len(req.Migrations) == 0 {
		return req.Failed(fmt.Errorf("no migration files were sent to apply"))
	}

	dir, cleanup, err := materialiseMigrations(req.Migrations)
	if err != nil {
		return req.Failed(err)
	}
	defer cleanup()

	db, err := e.connectAsOwner(ctx, target)
	if err != nil {
		return req.Failed(err)
	}
	defer db.Close()

	// Read what is pending before applying, so the report can name what ran.
	// Afterwards there is nothing left to enumerate.
	pending, err := db.PendingMigrations(ctx, dir)
	if err != nil {
		return req.Failed(fmt.Errorf("reading pending migrations: %w", err))
	}

	maxRisk := sqlschema.RiskLevel(req.MaxRisk)
	if maxRisk == "" {
		maxRisk = sqlschema.RiskSafe
	}

	report := &ApplyReport{}
	for _, migration := range pending {
		if riskExceeds(migration.Risk, maxRisk) {
			report.Blocked = append(report.Blocked, BlockedMigration{
				Name: migration.Name,
				Risk: string(migration.Risk),
			})
			continue
		}
		report.Applied = append(report.Applied, migration.Name)
	}

	if err := db.ApplyMigrationDir(ctx, dir, maxRisk); err != nil {
		return req.Failed(err)
	}

	e.logger.Info("applied migrations",
		zap.String("database", req.Database),
		zap.Int("applied", len(report.Applied)),
		zap.Int("blocked", len(report.Blocked)),
	)
	return &Response{Operation: req.Operation, Database: req.Database, OK: true, Apply: report}
}

func riskExceeds(risk sqlschema.RiskLevel, maxRisk sqlschema.RiskLevel) bool {
	return riskRank(risk) > riskRank(maxRisk)
}

func riskRank(risk sqlschema.RiskLevel) int {
	switch risk {
	case sqlschema.RiskBreaking:
		return 2
	case sqlschema.RiskRisky:
		return 1
	default:
		return 0
	}
}

// Writes the request's files into a fresh directory.
//
// A name that is anything but a bare file name is rejected rather than
// sanitised. A payload that can choose a path can write outside the directory,
// and a request that tries is worth refusing rather than quietly correcting.
func materialiseMigrations(files []MigrationFile) (string, func(), error) {
	dir, err := os.MkdirTemp("", "celerity-migrations-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("creating a migration directory: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }

	for _, file := range files {
		if file.Name == "" || file.Name != filepath.Base(file.Name) {
			cleanup()
			return "", func() {}, fmt.Errorf(
				"migration file name %q must be a bare file name", file.Name)
		}
		path := filepath.Join(dir, file.Name)
		if err := os.WriteFile(path, []byte(file.Content), 0600); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("writing migration %s: %w", file.Name, err)
		}
	}
	return dir, cleanup, nil
}

func (e *Executor) ping(ctx context.Context, req Request, target *Target) *Response {
	db, err := e.connectAsOwner(ctx, target)
	if err != nil {
		return req.Failed(err)
	}
	defer db.Close()
	return &Response{Operation: req.Operation, Database: req.Database, OK: true}
}

// Reads the application user's credential in whichever mode the database is
// configured for.
func (e *Executor) appUser(ctx context.Context, target *Target) (sqlschema.AppUser, error) {
	user := sqlschema.AppUser{
		Name:     target.AppUser,
		IAMAuth:  target.AppUserTokenAuth,
		Database: target.Database,
	}
	if user.Name == "" {
		return user, fmt.Errorf("no application user is configured for this database")
	}
	// Token authentication distributes no credential at all, so there is nothing
	// to read. The role still has to exist and hold the provider's grant, which
	// is why provisioning runs in both modes.
	if user.IAMAuth {
		return user, nil
	}

	if target.AppUserCredentialRef == "" {
		return user, fmt.Errorf(
			"database %q uses password authentication but no application user credential was configured",
			target.Database)
	}
	creds, err := e.credentials.Resolve(ctx, target.AppUserCredentialRef)
	if err != nil {
		return user, fmt.Errorf("reading the application user's credentials: %w", err)
	}
	user.Password = creds.Password
	return user, nil
}

// The owner/master role is the one migrations run as, and the only one that can
// grant over its own objects.
func (e *Executor) connectAsOwner(ctx context.Context, target *Target) (applier, error) {
	endpoint := dbconn.Endpoint{
		Engine:   target.Engine,
		Host:     target.Host,
		Port:     target.Port,
		Database: target.Database,
		// A managed database always speaks TLS, and the runner only ever talks
		// to one.
		TLS: true,
	}

	creds, err := e.ownerCredentials(ctx, endpoint, target)
	if err != nil {
		return nil, err
	}

	connStr, err := dbconn.URL(endpoint, creds)
	if err != nil {
		return nil, err
	}

	db, err := e.connect(ctx, target.Engine, connStr)
	if err != nil {
		// A connection failure here is nearly always a network rule, and the raw
		// driver error reports only that a dial timed out.
		return nil, fmt.Errorf(
			"connecting to %s:%s as the database owner: %w "+
				"(the runner must have a network path to the database)",
			target.Host, target.Port, err)
	}
	return db, nil
}

// The owner credential comes from the stored master credential, or from a
// platform-issued token where the database authenticates its master that way.
func (e *Executor) ownerCredentials(
	ctx context.Context,
	endpoint dbconn.Endpoint,
	target *Target,
) (dbconn.Credentials, error) {
	if target.OwnerCredentialRef == "" {
		return e.credentials.Token(ctx, endpoint, target.OwnerUser, target.Region)
	}
	creds, err := e.credentials.Resolve(ctx, target.OwnerCredentialRef)
	if err != nil {
		return dbconn.Credentials{}, fmt.Errorf("reading the database owner's credentials: %w", err)
	}
	return creds, nil
}

func connectToDatabase(_ context.Context, engine string, connStr string) (applier, error) {
	return sqlschema.NewApplier(engine, connStr, zap.NewNop())
}

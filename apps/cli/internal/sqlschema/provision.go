package sqlschema

import (
	"context"
	"fmt"
	"regexp"

	"go.uber.org/zap"
)

// AppUser describes the database role the application connects as.
//
// This is deliberately not the master user. The master can drop any table and
// read every database on the instance, and the RDS Proxy already holds it to
// authenticate on the application's behalf; handing it to every handler would
// undo the per-handler role work. Migrations keep running as the master, the
// application runs as this role, and provisioning is what connects the two.
type AppUser struct {
	// Name is the role to create. It is validated as an identifier, since
	// provisioning takes no free-form SQL.
	Name string

	// Password backs password authentication. Ignored in IAM mode.
	Password string

	// IAMAuth grants the engine's IAM authentication role instead of setting a
	// password, so the handler exchanges its execution role for a short-lived
	// token.
	IAMAuth bool

	// Database is the database the role is granted CONNECT on. Defaults to the
	// connected database when empty.
	Database string
}

// ProvisionReport records what provisioning actually changed, so a repeat run
// can honestly report that it did nothing.
type ProvisionReport struct {
	// User is the role that was provisioned.
	User string

	// Schema is the schema its object privileges were granted in.
	Schema string

	// RoleCreated is true when the role did not exist and was created.
	RoleCreated bool

	// AuthConfigured is true when a password was set or the IAM role granted.
	AuthConfigured bool

	// IAMAuth records which authentication mode was configured.
	IAMAuth bool

	// Grants lists the privileges applied, in the order they were applied. Safe
	// to print: identifiers only, never the password.
	Grants []string
}

// Changed reports whether provisioning altered anything beyond re-applying
// grants that were already in place. Grants are idempotent and cheap, so they
// are re-applied every run and do not count as a change.
func (r *ProvisionReport) Changed() bool {
	return r != nil && (r.RoleCreated || r.AuthConfigured)
}

// Render writes a human-readable summary of a provisioning run.
func (r *ProvisionReport) Render(w interface{ Write([]byte) (int, error) }) {
	if r == nil {
		return
	}
	verb := "already present"
	if r.RoleCreated {
		verb = "created"
	}
	mode := "password authentication"
	if r.IAMAuth {
		mode = "IAM authentication"
	}
	fmt.Fprintf(w, "  Application user %q %s, %s configured.\n", r.User, verb, mode)
	for _, grant := range r.Grants {
		fmt.Fprintf(w, "    granted %s\n", grant)
	}
}

// identifierPattern is what provisioning will accept as a role, schema or
// database name.
//
// Every engine allows far more than this once quoting is involved, and none of
// it is needed here, because these names come from a blueprint rather than from
// user input at the point of use. Refusing anything unusual is what keeps
// provisioning a fixed routine parameterised by identifiers rather than
// something that executes SQL it was handed.
var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

// Postgres's limit is 63 bytes and MySQL allows 64, so the stricter of the two
// applies everywhere and a name stays portable between engines.
const maxIdentifierLength = 63

// A name that passes this is safe to interpolate into DDL.
func validateIdentifier(kind string, name string) error {
	if name == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	if len(name) > maxIdentifierLength {
		return fmt.Errorf("%s name %q is longer than %d characters", kind, name, maxIdentifierLength)
	}
	if !identifierPattern.MatchString(name) {
		return fmt.Errorf(
			"%s name %q is not a plain identifier (letters, digits and underscores, not starting with a digit)",
			kind, name)
	}
	return nil
}

// ProvisionAppUser creates or repairs the application database role. It is
// idempotent, so it can run after every deploy, and after a database is restored
// or replaced, without a caller having to determine whether it is needed.
//
// It must be called on an Applier connected as the owner/master, the role that
// runs migrations, since it grants privileges over that role's objects. The
// default privileges it sets are attached to that same role, so tables created
// by later migrations are usable by the application without provisioning running
// again.
func (a *Applier) ProvisionAppUser(ctx context.Context, user AppUser) (*ProvisionReport, error) {
	if err := validateIdentifier("application user", user.Name); err != nil {
		return nil, err
	}
	if user.Database != "" {
		if err := validateIdentifier("database", user.Database); err != nil {
			return nil, err
		}
	}
	if !user.IAMAuth && user.Password == "" {
		return nil, fmt.Errorf(
			"application user %q uses password authentication but no password was supplied",
			user.Name)
	}

	schemaName, err := a.dialect.SchemaName(ctx, a.db)
	if err != nil {
		return nil, fmt.Errorf("resolving the managed schema: %w", err)
	}
	if err := validateIdentifier("schema", schemaName); err != nil {
		return nil, err
	}

	report, err := a.dialect.ProvisionAppUser(ctx, a.db, provisionRequest{
		User:     user,
		Schema:   schemaName,
		Database: user.Database,
	})
	if err != nil {
		return nil, err
	}

	a.logger.Info("provisioned application database user",
		zap.String("user", report.User),
		zap.Bool("roleCreated", report.RoleCreated),
		zap.Bool("iamAuth", report.IAMAuth),
	)
	return report, nil
}

// The validated input a dialect's provisioning routine works from. Every
// identifier in it has passed validateIdentifier, which is what allows the
// dialects to interpolate them into DDL.
type provisionRequest struct {
	User     AppUser
	Schema   string
	Database string
}

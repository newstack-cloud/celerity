package sqlschema

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// The role RDS grants IAM database authentication through. It exists only on
// RDS and Aurora Postgres, which is how provisioning tells a managed database
// from a plain one.
const pgIAMRole = "rds_iam"

// What the application is granted on the managed schema, which is to read and
// write rows and nothing structural. Migrations create and alter objects, and
// they run as the owner.
const appUserPrivileges = "SELECT, INSERT, UPDATE, DELETE"

// Identity and serial columns need the sequence granted as well as the table.
const appUserSequencePrivileges = "USAGE, SELECT"

// ProvisionAppUser creates or repairs the application role on Postgres.
//
// Every statement is written to be safe to repeat. The role is created only when
// absent, authentication is re-applied unconditionally (which is also how a
// rotated password reaches the database), and grants are idempotent by
// definition.
func (d postgresDialect) ProvisionAppUser(
	ctx context.Context,
	db *sql.DB,
	req provisionRequest,
) (*ProvisionReport, error) {
	name := req.User.Name
	quotedUser := d.QuoteIdentifier(name)

	database := req.Database
	if database == "" {
		if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database); err != nil {
			return nil, fmt.Errorf("reading the connected database name: %w", err)
		}
		if err := validateIdentifier("database", database); err != nil {
			return nil, err
		}
	}

	report := &ProvisionReport{User: name, Schema: req.Schema, IAMAuth: req.User.IAMAuth}

	exists, err := pgRoleExists(ctx, db, name)
	if err != nil {
		return nil, err
	}
	if !exists {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("CREATE ROLE %s WITH LOGIN", quotedUser)); err != nil {
			return nil, fmt.Errorf("creating role %q: %w", name, err)
		}
		report.RoleCreated = true
	}

	if err := d.configurePostgresAuth(ctx, db, req, quotedUser, report); err != nil {
		return nil, err
	}

	grants := []string{
		fmt.Sprintf("CONNECT ON DATABASE %s TO %s", d.QuoteIdentifier(database), quotedUser),
		fmt.Sprintf("USAGE ON SCHEMA %s TO %s", d.QuoteIdentifier(req.Schema), quotedUser),
		fmt.Sprintf("%s ON ALL TABLES IN SCHEMA %s TO %s",
			appUserPrivileges, d.QuoteIdentifier(req.Schema), quotedUser),
		fmt.Sprintf("%s ON ALL SEQUENCES IN SCHEMA %s TO %s",
			appUserSequencePrivileges, d.QuoteIdentifier(req.Schema), quotedUser),
	}
	for _, grant := range grants {
		if _, err := db.ExecContext(ctx, "GRANT "+grant); err != nil {
			return nil, fmt.Errorf("granting %s: %w", grant, err)
		}
		report.Grants = append(report.Grants, grant)
	}

	// Without this, every migration that creates a table would leave the
	// application unable to read it until provisioning ran again. The defaults
	// attach to the role executing this statement, which is the same role
	// migrations run as, so they apply to exactly the objects Celerity creates
	// and not to anything another role makes in the same schema.
	defaults := []string{
		fmt.Sprintf("%s ON TABLES TO %s", appUserPrivileges, quotedUser),
		fmt.Sprintf("%s ON SEQUENCES TO %s", appUserSequencePrivileges, quotedUser),
	}
	for _, def := range defaults {
		stmt := fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s",
			d.QuoteIdentifier(req.Schema), def)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("setting default privileges (%s): %w", def, err)
		}
		report.Grants = append(report.Grants, "default privileges: "+def)
	}

	return report, nil
}

func (d postgresDialect) configurePostgresAuth(
	ctx context.Context,
	db *sql.DB,
	req provisionRequest,
	quotedUser string,
	report *ProvisionReport,
) error {
	if req.User.IAMAuth {
		iamAvailable, err := pgRoleExists(ctx, db, pgIAMRole)
		if err != nil {
			return err
		}
		if !iamAvailable {
			return fmt.Errorf(
				"database %q is configured for IAM authentication but has no %s role, "+
					"which only exists on RDS and Aurora Postgres with IAM database authentication enabled",
				req.Database, pgIAMRole)
		}
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf("GRANT %s TO %s", pgIAMRole, quotedUser)); err != nil {
			return fmt.Errorf("granting %s to %q: %w", pgIAMRole, req.User.Name, err)
		}
		report.AuthConfigured = true
		return nil
	}

	// The password is the one value here that is not an identifier, so the
	// server quotes it rather than string-building doing so. format's %L applies
	// Postgres's own literal-quoting rules, including the escape-string prefix
	// when the value needs one.
	var stmt string
	err := db.QueryRowContext(ctx,
		"SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', $1::text, $2::text)",
		req.User.Name, req.User.Password,
	).Scan(&stmt)
	if err != nil {
		return fmt.Errorf("preparing the password statement for %q: %w", req.User.Name, err)
	}
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		// The statement carries the password, so it must never reach an error
		// message, a log line or a CI transcript.
		return fmt.Errorf("setting the password for %q: %w", req.User.Name, err)
	}
	report.AuthConfigured = true
	return nil
}

func pgRoleExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx, "SELECT 1 FROM pg_roles WHERE rolname = $1", name).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking whether role %q exists: %w", name, err)
	}
	return true, nil
}

package sqlschema

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// The host part of the application account. Handlers reach the database through
// the proxy from addresses that are not fixed, so the account is not
// host-restricted and security groups control the network path instead.
const mysqlAppUserHost = "%"

// The authentication plugin RDS uses for IAM database authentication.
const mysqlIAMPlugin = "AWSAuthenticationPlugin"

// ProvisionAppUser creates or repairs the application account on MySQL.
//
// MySQL grants privileges on a database with a wildcard, so unlike Postgres
// there is no separate default-privileges step. Tables created by later
// migrations are covered by the same grant.
func (d mysqlDialect) ProvisionAppUser(
	ctx context.Context,
	db *sql.DB,
	req provisionRequest,
) (*ProvisionReport, error) {
	name := req.User.Name
	account := fmt.Sprintf("'%s'@'%s'", name, mysqlAppUserHost)

	database := req.Database
	if database == "" {
		database = req.Schema
	}

	report := &ProvisionReport{User: name, Schema: req.Schema, IAMAuth: req.User.IAMAuth}

	exists, err := mysqlUserExists(ctx, db, name)
	if err != nil {
		return nil, err
	}
	if !exists {
		if _, err := db.ExecContext(ctx, "CREATE USER "+account); err != nil {
			return nil, fmt.Errorf("creating user %s: %w", account, err)
		}
		report.RoleCreated = true
	}

	if err := mysqlConfigureAuth(ctx, db, req, account, report); err != nil {
		return nil, err
	}

	grant := fmt.Sprintf("%s ON %s.* TO %s",
		appUserPrivileges, d.QuoteIdentifier(database), account)
	if _, err := db.ExecContext(ctx, "GRANT "+grant); err != nil {
		return nil, fmt.Errorf("granting %s: %w", grant, err)
	}
	report.Grants = append(report.Grants, grant)

	return report, nil
}

func mysqlConfigureAuth(
	ctx context.Context,
	db *sql.DB,
	req provisionRequest,
	account string,
	report *ProvisionReport,
) error {
	if req.User.IAMAuth {
		stmt := fmt.Sprintf("ALTER USER %s IDENTIFIED WITH %s AS 'RDS'", account, mysqlIAMPlugin)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf(
				"configuring IAM authentication for %s (the %s plugin exists only on RDS and Aurora MySQL): %w",
				account, mysqlIAMPlugin, err)
		}
		report.AuthConfigured = true
		return nil
	}

	literal, err := mysqlQuoteLiteral(ctx, db, req.User.Password)
	if err != nil {
		return err
	}
	// The statement carries the password, so nothing built here may appear in an
	// error message.
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf("ALTER USER %s IDENTIFIED BY %s", account, literal)); err != nil {
		return fmt.Errorf("setting the password for %s: %w", account, err)
	}
	report.AuthConfigured = true
	return nil
}

func mysqlUserExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM mysql.user WHERE user = ? AND host = ?",
		name, mysqlAppUserHost,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking whether user %q exists: %w", name, err)
	}
	return count > 0, nil
}

// Quotes a string for interpolation into DDL that cannot take a placeholder.
//
// MySQL has no server-side quoting function to delegate to the way Postgres's
// format does, so the escaping is done here, and it depends on the session's
// sql_mode. Under NO_BACKSLASH_ESCAPES a backslash is an ordinary character and
// escaping with one would change the value being stored. Rather than guess, that
// mode is refused.
func mysqlQuoteLiteral(ctx context.Context, db *sql.DB, value string) (string, error) {
	var mode string
	if err := db.QueryRowContext(ctx, "SELECT @@session.sql_mode").Scan(&mode); err != nil {
		return "", fmt.Errorf("reading sql_mode: %w", err)
	}
	if strings.Contains(strings.ToUpper(mode), "NO_BACKSLASH_ESCAPES") {
		return "", fmt.Errorf(
			"cannot set a password while the session runs with NO_BACKSLASH_ESCAPES; " +
				"remove it from sql_mode on the database's parameter group, or use IAM authentication")
	}

	replacer := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		"\x00", `\0`,
		"\n", `\n`,
		"\r", `\r`,
		"\x1a", `\Z`,
	)
	return "'" + replacer.Replace(value) + "'", nil
}

// Package dbconn turns an endpoint plus a resolved credential into a
// connection string for a SQL engine.
//
// It exists because two places need exactly this and must not drift: the CLI
// connecting directly to a reachable database, and the maintenance runner
// connecting from inside the network to one that is not. A difference between
// them, a missing sslmode or a MySQL DSN built like a URL, would show up only on
// deployed infrastructure, and only on one of the paths.
//
// Nothing here is cloud-specific. Where a credential comes from is a provider
// concern, behind the CredentialSource interface; the AWS implementation is in
// the awscreds sub-package.
package dbconn

import (
	"context"
	"fmt"
	"net"
	"net/url"

	gomysql "github.com/go-sql-driver/mysql"
)

// Endpoint is where a database is and which database on it to open.
type Endpoint struct {
	// Engine is "postgres" (the default when empty) or "mysql".
	Engine string

	Host     string
	Port     string
	Database string

	// TLS requires an encrypted connection. Always true against a managed
	// database, and false against a local container, so it is explicit rather
	// than assumed.
	TLS bool
}

// Credentials are a resolved username and password. In token-based
// authentication (AWS IAM, GCP's IAM database authentication) the token takes
// the password's place. The wire protocol is the same either way, which is why
// there is one type here rather than two.
type Credentials struct {
	Username string
	Password string
}

// CredentialSource resolves the credential a reference points at.
//
// A reference is whatever the provider's secret store calls one, be it an ARN, a
// projects/…/secrets/… resource name or a Key Vault URI, and it is opaque here.
// Implementations live per provider so this package, and everything built on it,
// stays cloud-neutral.
type CredentialSource interface {
	// Resolve reads the credential a reference points at.
	Resolve(ctx context.Context, reference string) (Credentials, error)

	// Token issues a short-lived database token for a user, for providers whose
	// databases accept the platform's own identity in place of a password. It
	// returns an error where the provider has no such mechanism.
	Token(ctx context.Context, endpoint Endpoint, user string, region string) (Credentials, error)
}

// URL builds a connection string for the endpoint's engine.
func URL(endpoint Endpoint, creds Credentials) (string, error) {
	switch endpoint.Engine {
	case "postgres", "":
		return postgresURL(endpoint, creds), nil
	case "mysql":
		return mysqlDSN(endpoint, creds), nil
	default:
		return "", fmt.Errorf("unsupported SQL engine %q", endpoint.Engine)
	}
}

func postgresURL(endpoint Endpoint, creds Credentials) string {
	sslMode := "disable"
	if endpoint.TLS {
		sslMode = "require"
	}
	u := url.URL{
		Scheme: "postgres",
		// A generated password can contain anything, and url.UserPassword
		// percent-encodes where a formatted string would not.
		User:     url.UserPassword(creds.Username, creds.Password),
		Host:     net.JoinHostPort(endpoint.Host, endpoint.Port),
		Path:     "/" + endpoint.Database,
		RawQuery: "sslmode=" + sslMode,
	}
	return u.String()
}

func mysqlDSN(endpoint Endpoint, creds Credentials) string {
	cfg := gomysql.NewConfig()
	cfg.User = creds.Username
	cfg.Passwd = creds.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(endpoint.Host, endpoint.Port)
	cfg.DBName = endpoint.Database
	cfg.ParseTime = true
	if endpoint.TLS {
		cfg.TLSConfig = "true"
		// A token authenticates in cleartext, which is what the encrypted
		// connection above makes acceptable. Password authentication does not
		// need it but is unharmed by it, so the flag follows TLS rather than the
		// authentication mode.
		cfg.AllowCleartextPasswords = true
	}
	return cfg.FormatDSN()
}

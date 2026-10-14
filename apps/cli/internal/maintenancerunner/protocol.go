// Package maintenancerunner defines the contract between the Celerity CLI and
// the maintenance runner, and implements the runner side of it.
//
// A Celerity SQL database sits on a private network, so nothing outside it can
// open a connection, including the machine the operator runs `celerity schema`
// from. The runner closes that gap without asking anyone to build network
// access. It is deployed inside the network, where it has a data-plane path to
// the database, and invoked over the provider's control plane, which is
// reachable with the same credentials that just ran the deploy.
//
// What crosses the boundary is the *operation*, not the connection. The CLI
// requests "provision the application user on ordersDb"; the runner performs it
// against a local connection and returns a report. Provisioning and migration
// run through the same internal/sqlschema package in both places, so there is
// no second implementation to keep in step.
//
// # Provider boundaries
//
// Everything in this package is cloud-neutral: the request and response format,
// the target configuration, dispatch and the operations themselves. Two things
// are not, and both live outside it. How the runner is hosted and invoked is the
// awsrunner sub-package's business, and where credentials come from is a
// dbconn.CredentialSource's. Adding a provider means adding those two, not
// changing this.
//
// # Why the request does not describe the database
//
// A request names a database; it does not carry a host, a port or a credential.
// Everything needed to connect is stamped into the runner's own configuration
// at deploy time (see TargetsEnvVar). A caller therefore cannot point the
// runner at a database of their choosing, and the set of things it will ever
// connect to is fixed by the blueprint rather than by whoever holds permission
// to invoke it.
package maintenancerunner

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
)

// Operations the runner accepts.
//
// Provisioning and ping are named routines that execute no SQL they are handed.
// Apply is different and deliberately so, because migrations *are* SQL written
// by the author of the blueprint and there is no way to run them inside the
// network without carrying them in. What bounds it is the invoking identity
// rather than the payload. The runner's own permissions reach exactly the
// databases the blueprint gave it, and a caller must already hold invoke
// permission on it, which is the same permission that could otherwise redeploy
// the function outright. The migration set is also checksum-verified against
// atlas.sum
// before anything runs, so a payload that disagrees with the directory it
// claims to be is rejected rather than applied.
const (
	// OpProvisionUser creates or repairs the application database user.
	OpProvisionUser = "provision-user"

	// OpPing checks the runner can reach and authenticate to a database. It
	// exists so a deploy can verify the path works without changing anything.
	OpPing = "ping"

	// OpApply applies pending migrations, as the database owner.
	OpApply = "apply"
)

// TargetsEnvVar is the environment variable the transformer stamps the runner's
// database targets into, as a JSON object keyed by the abstract database name.
//
// An environment variable rather than a config store because it is the one
// piece of configuration every host platform supplies the same way. AWS Lambda
// caps environment variables at 4KB in total, which holds on the order of ten
// databases; beyond that the targets need to move to a parameter, which is a
// change to this constant's contract and nothing else.
const TargetsEnvVar = "CELERITY_MAINTENANCE_TARGETS"

// Request is the Lambda invocation payload.
type Request struct {
	// Operation is one of the Op constants.
	Operation string `json:"operation"`

	// Database is the abstract name of the celerity/sqlDatabase to act on, as
	// written in the blueprint.
	Database string `json:"database"`

	// Migrations carries the migration directory for OpApply, because the files
	// live on the operator's machine and the runner has no access to it.
	//
	// A synchronous Lambda invocation caps its payload at 6MB, which is far more
	// than a migration directory of text files, but ApplyLimits is checked
	// before invoking so an oversized set is reported as such rather than as a
	// truncated request.
	Migrations []MigrationFile `json:"migrations,omitempty"`

	// MaxRisk is the highest migration risk level the operator authorised, from
	// the same --allow-risky and --allow-breaking flags the local path uses. An
	// empty value permits only safe migrations.
	MaxRisk string `json:"maxRisk,omitempty"`
}

// MigrationFile is one file from the migration directory, including atlas.sum.
type MigrationFile struct {
	// Name is the base file name, never a path. The runner writes these into a
	// directory of its own, and a separator here would let a payload write
	// outside it.
	Name string `json:"name"`

	// Content is the file's bytes as text. Migration files are SQL and the sum
	// file is text, so nothing here needs encoding.
	Content string `json:"content"`
}

// Response is the Lambda invocation result.
//
// A failed operation is a successful invocation with OK false rather than a
// Lambda error. The distinction matters because a Lambda error gives the CLI a
// stack trace and no report, while this gives it something to print.
type Response struct {
	Operation string `json:"operation"`
	Database  string `json:"database"`
	OK        bool   `json:"ok"`

	// Error describes why the operation failed, in terms an operator can act on.
	Error string `json:"error,omitempty"`

	// Provision is the report from OpProvisionUser.
	Provision *sqlschema.ProvisionReport `json:"provision,omitempty"`

	// Apply is the report from OpApply.
	Apply *ApplyReport `json:"apply,omitempty"`
}

// ApplyReport describes what a migration run did, in the same terms the local
// path prints so an operator cannot tell from the output which route was taken.
type ApplyReport struct {
	// Applied names the migrations that ran, in order. Empty when the database
	// was already up to date.
	Applied []string `json:"applied,omitempty"`

	// Blocked names migrations that were left unapplied because their risk
	// exceeds what the operator authorised, with the risk that stopped them.
	Blocked []BlockedMigration `json:"blocked,omitempty"`
}

// BlockedMigration is a pending migration held back by the risk gate.
type BlockedMigration struct {
	Name string `json:"name"`
	Risk string `json:"risk"`
}

// Changed reports whether the run altered the database.
func (r *ApplyReport) Changed() bool {
	return r != nil && len(r.Applied) > 0
}

// Failed builds an unsuccessful response for a request.
func (r Request) Failed(err error) *Response {
	return &Response{
		Operation: r.Operation,
		Database:  r.Database,
		OK:        false,
		Error:     err.Error(),
	}
}

// Target is everything the runner needs to reach one database as its
// owner/master, and to determine what to provision on it.
//
// Only references travel here, never a credential. What a reference looks like
// is the provider's business (an ARN on AWS today), and the runner resolves it
// through a dbconn.CredentialSource. Its deployed identity is granted exactly
// the references listed here for each database, so what it can read is visible
// in the deployed policy rather than in a payload.
type Target struct {
	// Engine is "postgres" or "mysql".
	Engine string `json:"engine"`

	// Host and Port are the connection-pool endpoint where there is one,
	// otherwise the database's own endpoint.
	Host string `json:"host"`
	Port string `json:"port"`

	// Database is the database name to connect to.
	Database string `json:"database"`

	// OwnerCredentialRef points at the master/owner credential, the role
	// migrations run as. Empty when the database authenticates its owner with a
	// platform-issued token instead.
	OwnerCredentialRef string `json:"ownerCredentialRef,omitempty"`

	// OwnerUser and Region are what a token is issued for, when
	// OwnerCredentialRef is empty.
	OwnerUser string `json:"ownerUser,omitempty"`
	Region    string `json:"region,omitempty"`

	// AppUser is the role the application connects as.
	AppUser string `json:"appUser"`

	// AppUserCredentialRef points at the application user's generated password.
	// Empty when the application authenticates with a token, which needs no
	// stored credential.
	AppUserCredentialRef string `json:"appUserCredentialRef,omitempty"`

	// AppUserTokenAuth selects platform-issued token authentication (AWS IAM
	// database authentication, and its equivalents) for the application user.
	AppUserTokenAuth bool `json:"appUserTokenAuth,omitempty"`
}

// Targets is the runner's deploy-time configuration, keyed by abstract database
// name.
type Targets map[string]*Target

// LoadTargets reads the runner's targets from its environment.
func LoadTargets() (Targets, error) {
	raw := os.Getenv(TargetsEnvVar)
	if raw == "" {
		return nil, fmt.Errorf(
			"%s is not set; the runner was deployed without any database targets", TargetsEnvVar)
	}
	return ParseTargets(raw)
}

// ParseTargets parses the targets JSON.
func ParseTargets(raw string) (Targets, error) {
	var targets Targets
	if err := json.Unmarshal([]byte(raw), &targets); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", TargetsEnvVar, err)
	}
	return targets, nil
}

// Names returns the databases this runner serves, sorted.
func (t Targets) Names() []string {
	names := make([]string, 0, len(t))
	for name := range t {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Lookup returns the target for a database. A name that does not match is far
// more often a stale deploy than a typo, so the error lists the databases this
// runner is configured for.
func (t Targets) Lookup(database string) (*Target, error) {
	if target, ok := t[database]; ok {
		return target, nil
	}
	known := make([]string, 0, len(t))
	for name := range t {
		known = append(known, name)
	}
	if len(known) == 0 {
		return nil, fmt.Errorf("this runner has no database targets configured")
	}
	return nil, fmt.Errorf(
		"no database named %q on this runner (it serves: %v); the deployed instance may predate the database",
		database, known)
}

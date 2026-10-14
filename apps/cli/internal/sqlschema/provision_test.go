package sqlschema

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Provisioning interpolates names into DDL that cannot take placeholders, so
// what it accepts as a name is the boundary that keeps it a fixed routine
// rather than something that runs SQL it was handed.
func TestValidateIdentifier(t *testing.T) {
	valid := []string{"celerity_app", "app$1", "_leading", "A", strings.Repeat("a", 63)}
	for _, name := range valid {
		assert.NoError(t, validateIdentifier("application user", name), name)
	}

	rejected := map[string]string{
		"":                          "is required",
		"1app":                      "not a plain identifier",
		"app-user":                  "not a plain identifier",
		"app user":                  "not a plain identifier",
		`app"; DROP DATABASE x; --`: "not a plain identifier",
		"app\x00":                   "not a plain identifier",
		strings.Repeat("a", 64):     "longer than 63",
	}
	for name, want := range rejected {
		err := validateIdentifier("application user", name)
		require.Error(t, err, "%q should be rejected", name)
		assert.Contains(t, err.Error(), want)
	}
}

// A database in password mode with no password would produce a role nothing can
// authenticate as, which fails later and somewhere less obvious.
func TestProvisionAppUserRequiresACredential(t *testing.T) {
	applier := &Applier{}

	_, err := applier.ProvisionAppUser(context.Background(), AppUser{Name: "celerity_app"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no password was supplied")
}

func TestProvisionAppUserValidatesNames(t *testing.T) {
	applier := &Applier{}

	_, err := applier.ProvisionAppUser(context.Background(), AppUser{
		Name: "celerity app", Password: "x",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "application user")
}

// Grants are re-applied on every run because they are idempotent and cheap, so
// applying them is not itself a change worth reporting.
func TestProvisionReportChanged(t *testing.T) {
	assert.False(t, (&ProvisionReport{Grants: []string{"USAGE ON SCHEMA"}}).Changed())
	assert.True(t, (&ProvisionReport{RoleCreated: true}).Changed())
	assert.True(t, (&ProvisionReport{AuthConfigured: true}).Changed())
	assert.False(t, (*ProvisionReport)(nil).Changed())
}

func TestProvisionReportRenderOmitsTheCredential(t *testing.T) {
	var out strings.Builder
	report := &ProvisionReport{
		User:           "celerity_app",
		AuthConfigured: true,
		Grants:         []string{`USAGE ON SCHEMA "public" TO "celerity_app"`},
	}

	report.Render(&out)

	assert.Contains(t, out.String(), `"celerity_app" already present, password authentication configured`)
	assert.Contains(t, out.String(), "granted USAGE ON SCHEMA")
}

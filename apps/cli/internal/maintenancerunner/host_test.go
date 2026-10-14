package maintenancerunner

import (
	"context"
	"testing"

	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeHost struct {
	name     string
	detected bool
	served   bool
}

func (f *fakeHost) Name() string { return f.name }
func (f *fakeHost) Detect() bool { return f.detected }

func (f *fakeHost) Credentials() dbconn.CredentialSource {
	return &fakeCredentials{stored: map[string]dbconn.Credentials{}}
}

func (f *fakeHost) Serve(_ Handler, _ *zap.Logger) error {
	f.served = true
	return nil
}

// withHosts replaces the registry for one test. The registry is package-level
// because hosts register themselves from init, which is the point, but it does
// mean a test has to put it back.
func withHosts(t *testing.T, registered ...Host) {
	t.Helper()
	previous := hosts
	hosts = map[string]Host{}
	for _, host := range registered {
		RegisterHost(host)
	}
	t.Cleanup(func() { hosts = previous })
}

func TestRunsOnTheHostItDetects(t *testing.T) {
	aws := &fakeHost{name: "aws", detected: true}
	other := &fakeHost{name: "elsewhere"}
	withHosts(t, aws, other)
	t.Setenv(TargetsEnvVar, `{"ordersDb":{"engine":"postgres","host":"h","port":"5432"}}`)

	require.NoError(t, Run(zap.NewNop()))

	assert.True(t, aws.served)
	assert.False(t, other.served, "only the detected host serves")
}

// The override exists for the case detection cannot cover, somewhere that looks
// like nothing in particular, such as a container during a test.
func TestTheHostCanBeNamedExplicitly(t *testing.T) {
	aws := &fakeHost{name: "aws", detected: true}
	other := &fakeHost{name: "elsewhere"}
	withHosts(t, aws, other)
	t.Setenv(TargetsEnvVar, `{"ordersDb":{"engine":"postgres","host":"h","port":"5432"}}`)
	t.Setenv(HostEnvVar, "elsewhere")

	require.NoError(t, Run(zap.NewNop()))

	assert.True(t, other.served)
	assert.False(t, aws.served, "the override wins over detection")
}

// Both startup failures name what the binary was actually built with, because
// the alternative is an operator guessing at a runner that exits immediately.
func TestAnUnknownHostNameListsWhatWasBuiltIn(t *testing.T) {
	withHosts(t, &fakeHost{name: "aws", detected: true})
	t.Setenv(HostEnvVar, "gcloud")

	err := Run(zap.NewNop())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gcloud")
	assert.Contains(t, err.Error(), "[aws]")
}

func TestNoDetectedHostIsAStartupFailure(t *testing.T) {
	withHosts(t, &fakeHost{name: "aws"})

	err := Run(zap.NewNop())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not tell which platform")
	assert.Contains(t, err.Error(), HostEnvVar)
}

// Targets are deploy-time configuration: a bad one should stop the runner at
// startup, not on whichever invocation lands on a fresh execution environment.
func TestMisconfiguredTargetsFailAtStartup(t *testing.T) {
	host := &fakeHost{name: "aws", detected: true}
	withHosts(t, host)
	t.Setenv(TargetsEnvVar, "not json")

	err := Run(zap.NewNop())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "misconfigured")
	assert.False(t, host.served)
}

func TestTargetNamesAreSorted(t *testing.T) {
	targets := Targets{"ordersDb": {}, "auditDb": {}}

	assert.Equal(t, []string{"auditDb", "ordersDb"}, targets.Names())
}

// The handler the host is given is the executor's, so what a host serves is
// exactly what the CLI's requests reach.
func TestTheHostIsGivenTheExecutorsHandler(t *testing.T) {
	var handled *Response
	host := &handlerCapturingHost{
		fakeHost: fakeHost{name: "aws", detected: true},
		call: func(handle Handler) {
			handled = handle(context.Background(), Request{
				Operation: "execute-sql", Database: "ordersDb",
			})
		},
	}
	withHosts(t, host)
	t.Setenv(TargetsEnvVar, `{"ordersDb":{"engine":"postgres","host":"h","port":"5432"}}`)

	require.NoError(t, Run(zap.NewNop()))

	require.NotNil(t, handled)
	assert.False(t, handled.OK)
	assert.Contains(t, handled.Error, "unknown operation")
}

type handlerCapturingHost struct {
	fakeHost
	call func(handle Handler)
}

func (h *handlerCapturingHost) Serve(handle Handler, _ *zap.Logger) error {
	h.served = true
	h.call(handle)
	return nil
}

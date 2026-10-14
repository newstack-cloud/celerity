package commands

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Replays a scripted sequence of responses, so a test can describe a database
// that is not ready and then becomes ready.
type scriptedInvoker struct {
	responses []*maintenancerunner.Response
	err       error
	requests  []maintenancerunner.Request
}

func (s *scriptedInvoker) Invoke(
	_ context.Context,
	_ *state.InstanceState,
	req maintenancerunner.Request,
) (*maintenancerunner.Response, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	next := s.responses[0]
	if len(s.responses) > 1 {
		s.responses = s.responses[1:]
	}
	return next, nil
}

func (s *scriptedInvoker) Deployed(_ *state.InstanceState) bool { return true }

func notReady(message string) *maintenancerunner.Response {
	return &maintenancerunner.Response{Operation: maintenancerunner.OpPing, OK: false, Error: message}
}

func ready() *maintenancerunner.Response {
	return &maintenancerunner.Response{Operation: maintenancerunner.OpPing, OK: true}
}

func withFastReadyPolling(t *testing.T) {
	t.Helper()
	previous := databaseReadyPollInterval
	databaseReadyPollInterval = time.Millisecond
	t.Cleanup(func() { databaseReadyPollInterval = previous })
}

// A connection pool provisioned with the deployment reports itself available
// while it is still scaling, and connections through it fail at the TLS layer.
// Running schema apply straight after a deploy is the normal thing to do, so
// this waits rather than reporting a broken deployment.
func TestWaitsForAConnectionPoolThatIsStillScaling(t *testing.T) {
	withFastReadyPolling(t)
	invoker := &scriptedInvoker{responses: []*maintenancerunner.Response{
		notReady("pinging postgres: tls error: EOF"),
		notReady("pinging postgres: tls error: EOF"),
		ready(),
	}}
	out := &bytes.Buffer{}

	err := waitForDatabaseReady(context.Background(), out, invoker, &state.InstanceState{}, "ordersDb")

	require.NoError(t, err)
	assert.Len(t, invoker.requests, 3)
	assert.Contains(t, out.String(), "Waiting for ordersDb")
	assert.Contains(t, out.String(), "accepting connections")
}

// A database that answers immediately must not announce a wait that did not
// happen, which is the common case and should be silent.
func TestSaysNothingWhenTheDatabaseIsAlreadyReady(t *testing.T) {
	withFastReadyPolling(t)
	invoker := &scriptedInvoker{responses: []*maintenancerunner.Response{ready()}}
	out := &bytes.Buffer{}

	err := waitForDatabaseReady(context.Background(), out, invoker, &state.InstanceState{}, "ordersDb")

	require.NoError(t, err)
	assert.Len(t, invoker.requests, 1)
	assert.Empty(t, out.String())
}

// Waiting cannot fix a credential or configuration problem, so it fails on the
// first attempt instead of retrying for five minutes and then saying the same
// thing.
func TestDoesNotWaitOutAFailureWaitingCannotFix(t *testing.T) {
	withFastReadyPolling(t)
	invoker := &scriptedInvoker{responses: []*maintenancerunner.Response{
		notReady("reading the database owner's credentials: access denied"),
	}}

	err := waitForDatabaseReady(context.Background(), &bytes.Buffer{}, invoker, &state.InstanceState{}, "ordersDb")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
	assert.Len(t, invoker.requests, 1, "a failure waiting cannot fix must not be retried")
}

func TestAnUnreachableRunnerIsReportedImmediately(t *testing.T) {
	withFastReadyPolling(t)
	invoker := &scriptedInvoker{err: errors.New("lambda: access denied")}

	err := waitForDatabaseReady(context.Background(), &bytes.Buffer{}, invoker, &state.InstanceState{}, "ordersDb")

	assert.ErrorContains(t, err, "lambda: access denied")
}

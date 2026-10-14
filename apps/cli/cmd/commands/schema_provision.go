package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner"

	// Registers the AWS maintenance runner invoker. Providers are wired in here
	// rather than in the maintenancerunner package so it stays cloud-neutral.
	_ "github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner/awsrunner"
)

// provisionAppUser makes sure the application's database role exists before
// migrations run.
//
// It runs on every apply rather than once, because it is idempotent and cheap
// and the alternative is a class of failure nobody can diagnose. A database that
// was restored from a snapshot, replaced, or deployed before this existed has an
// application credential that authenticates as nobody, and the symptom is every
// handler failing to connect long after the deploy reported success.
//
// A missing runner is not an error. An instance whose database is reachable
// without one, or one deployed before runners existed, still migrates. It
// provisions nothing and reports as much.
func provisionAppUser(
	ctx context.Context,
	out io.Writer,
	instance *state.InstanceState,
	deployTarget string,
	sqlDatabaseName string,
	via string,
) error {
	invoker, err := maintenancerunner.InvokerFor(deployTarget)
	if err != nil {
		if via == viaRunner {
			return err
		}
		return nil
	}

	if !invoker.Deployed(instance) {
		if via == viaRunner {
			return fmt.Errorf(
				"--via runner was requested but this instance has no deployed maintenance runner")
		}
		return nil
	}

	if err := waitForDatabaseReady(ctx, out, invoker, instance, sqlDatabaseName); err != nil {
		return err
	}

	fmt.Fprintf(out, "Checking the application database user on %s...\n", sqlDatabaseName)

	resp, err := invoker.Invoke(ctx, instance, maintenancerunner.Request{
		Operation: maintenancerunner.OpProvisionUser,
		Database:  sqlDatabaseName,
	})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("provisioning the application database user: %s", resp.Error)
	}

	if resp.Provision.Changed() {
		resp.Provision.Render(out)
	} else {
		fmt.Fprintf(out, "  Application user %q is already provisioned.\n", resp.Provision.User)
	}
	return nil
}

// How a maintenance operation reaches the database.
const (
	// viaAuto uses the runner when the instance has one, and connects directly
	// otherwise. The default, and right for nearly everyone.
	viaAuto = "auto"

	// viaRunner insists on the runner, and fails when there is none. Useful in
	// CI, where silently falling back to a direct connection that then times out
	// is a worse failure than saying so.
	viaRunner = "runner"

	// viaDirect ignores any runner and connects from here.
	viaDirect = "direct"
)

func validateVia(via string) error {
	switch via {
	case viaAuto, viaRunner, viaDirect:
		return nil
	default:
		return fmt.Errorf("invalid --via %q (expected %s, %s or %s)",
			via, viaAuto, viaRunner, viaDirect)
	}
}

// How long to wait for a database to start accepting connections, and how often
// to ask.
//
// A connection pool provisioned moments earlier is the case this exists for. An
// AWS RDS proxy reports itself available while its target is still
// PENDING_PROXY_CAPACITY, and connections through it fail with a TLS error that
// mentions nothing about capacity. Five minutes covers the scaling seen in
// practice, which is two to three.
//
// These are variables rather than constants so tests can drive the loop without
// waiting on real time. Nothing outside tests reassigns them.
var (
	databaseReadyTimeout      = 5 * time.Minute
	databaseReadyPollInterval = 15 * time.Second
)

// waitForDatabaseReady blocks until the runner can reach and authenticate to the
// database, or the wait is exhausted.
//
// Running `celerity schema apply` straight after a deploy is the normal thing to
// do, and it failed against a database whose connection pool had not finished
// scaling. That reads as a broken deployment rather than as one that is not
// ready yet, so the wait exists to tell the two apart.
//
// The check is the runner's own ping, which connects exactly as the operations
// that follow do. Anything waiting cannot fix, such as a missing target or an
// unreadable credential, fails on the first attempt rather than being retried
// for five minutes.
func waitForDatabaseReady(
	ctx context.Context,
	out io.Writer,
	invoker maintenancerunner.Invoker,
	instance *state.InstanceState,
	sqlDatabaseName string,
) error {
	deadline := time.Now().Add(databaseReadyTimeout)
	announced := false

	for attempt := 1; ; attempt++ {
		resp, err := invoker.Invoke(ctx, instance, maintenancerunner.Request{
			Operation: maintenancerunner.OpPing,
			Database:  sqlDatabaseName,
		})
		if err != nil {
			return err
		}
		if resp.OK {
			if announced {
				fmt.Fprintf(out, "  %s is accepting connections.\n", sqlDatabaseName)
			}
			return nil
		}

		if !isDatabaseNotReadyYet(resp.Error) {
			return fmt.Errorf("reaching %s: %s", sqlDatabaseName, resp.Error)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(
				"%s did not start accepting connections within %s; the last attempt reported: %s",
				sqlDatabaseName, databaseReadyTimeout, resp.Error)
		}

		if !announced {
			fmt.Fprintf(out,
				"Waiting for %s to accept connections (a connection pool provisioned with the deployment can take a few minutes to scale)...\n",
				sqlDatabaseName)
			announced = true
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(databaseReadyPollInterval):
		}
	}
}

// isDatabaseNotReadyYet reports whether a failure to reach the database is one
// that waiting can resolve.
//
// A pool that is still scaling refuses the connection at the TLS layer, so the
// error names neither capacity nor readiness; matching the shape of the failure
// is the only signal available. A credential or configuration problem produces a
// different error and is not retried, because waiting cannot fix it.
func isDatabaseNotReadyYet(message string) bool {
	lowered := strings.ToLower(message)
	for _, fragment := range []string{
		"tls error",
		"eof",
		"connection refused",
		"connection reset",
		"i/o timeout",
		"dial tcp",
		"no such host",
	} {
		if strings.Contains(lowered, fragment) {
			return true
		}
	}
	return false
}

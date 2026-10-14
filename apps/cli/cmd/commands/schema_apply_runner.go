package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner"
	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
)

// maxRunnerMigrationPayload bounds what is sent in one invocation.
//
// A synchronous Lambda invocation caps its request at 6MB. Checking here means
// an oversized migration set is reported as one, rather than as whatever the
// provider reports when it truncates or rejects the payload. The margin leaves
// room for the rest of the request and the JSON encoding of the files.
const maxRunnerMigrationPayload = 5 << 20

// applyMigrationsViaRunner applies pending migrations through the deployed
// maintenance runner, returning whether it handled the operation.
//
// A database on a private network cannot be reached from the operator's machine,
// which is the whole reason the runner exists. The CLI moves the operation rather
// than the connection. When there is no runner, this reports that it did nothing
// and the caller falls back to connecting directly, except under --via runner,
// where being unable to use it is the error.
func applyMigrationsViaRunner(
	ctx context.Context,
	out io.Writer,
	instance *state.InstanceState,
	deployTarget string,
	resource sqlschema.DatabaseResource,
	via string,
	maxRisk sqlschema.RiskLevel,
	confirm func() error,
) (bool, error) {
	if via == viaDirect {
		return false, nil
	}

	invoker, err := maintenancerunner.InvokerFor(deployTarget)
	if err != nil {
		if via == viaRunner {
			return false, err
		}
		return false, nil
	}
	if !invoker.Deployed(instance) {
		if via == viaRunner {
			return false, fmt.Errorf(
				"--via runner was requested but this instance has no deployed maintenance runner")
		}
		return false, nil
	}

	files, err := readMigrationDir(resource.MigrationsPath)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		fmt.Fprintf(out, "No migration files for %s.\n", resource.ResourceName)
		return true, nil
	}

	// What is pending can only be known by the side that can reach the database,
	// so unlike the direct path there is no list to show before confirming. The
	// report afterwards names exactly what ran.
	fmt.Fprintf(out,
		"Applying migrations for %s through the maintenance runner (risk up to: %s).\n",
		resource.ResourceName, maxRisk)
	if confirm != nil {
		if err := confirm(); err != nil {
			return false, err
		}
	}

	resp, err := invoker.Invoke(ctx, instance, maintenancerunner.Request{
		Operation:  maintenancerunner.OpApply,
		Database:   resource.ResourceName,
		Migrations: files,
		MaxRisk:    string(maxRisk),
	})
	if err != nil {
		return false, err
	}
	if !resp.OK {
		return false, fmt.Errorf("applying migrations for %s: %s", resource.ResourceName, resp.Error)
	}

	renderRunnerApplyReport(out, resource.ResourceName, resp.Apply)
	return true, nil
}

// renderRunnerApplyReport prints what the runner did, in the same terms the
// local path uses.
func renderRunnerApplyReport(out io.Writer, resourceName string, report *maintenancerunner.ApplyReport) {
	if report == nil || (len(report.Applied) == 0 && len(report.Blocked) == 0) {
		fmt.Fprintf(out, "No pending migrations for %s.\n", resourceName)
		return
	}

	for _, name := range report.Applied {
		fmt.Fprintf(out, "  - %s  applied\n", name)
	}
	for _, blocked := range report.Blocked {
		fmt.Fprintf(out, "  - %s  held back  [risk: %s]\n", blocked.Name, blocked.Risk)
	}

	if len(report.Applied) > 0 {
		fmt.Fprintf(out, "Applied %d migration(s) for %s.\n", len(report.Applied), resourceName)
	}
	if len(report.Blocked) > 0 {
		fmt.Fprintf(out,
			"%d migration(s) were held back by their risk level; re-run with --allow-risky or --allow-breaking once understood.\n",
			len(report.Blocked))
	}
}

// readMigrationDir reads the migration files to send to the runner.
//
// Only the top level is read, and only the files a migration directory is made
// of, which are the .sql migrations and atlas.sum. Sending anything else would
// put files on the runner that the migration machinery does not expect, and the
// checksum covers exactly this set.
func readMigrationDir(dir string) ([]maintenancerunner.MigrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading the migration directory %s: %w", dir, err)
	}

	files := make([]maintenancerunner.MigrationFile, 0, len(entries))
	total := 0
	for _, entry := range entries {
		if entry.IsDir() || !isMigrationDirFile(entry.Name()) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading migration %s: %w", entry.Name(), err)
		}
		total += len(content)
		if total > maxRunnerMigrationPayload {
			return nil, fmt.Errorf(
				"the migration directory is larger than %dMB, which is more than one runner invocation can carry; "+
					"apply the older migrations from inside the network, or squash them",
				maxRunnerMigrationPayload>>20)
		}
		files = append(files, maintenancerunner.MigrationFile{
			Name:    entry.Name(),
			Content: string(content),
		})
	}

	// Sorted so the payload is stable between runs, which makes two invocations
	// of the same directory comparable in a log.
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func isMigrationDirFile(name string) bool {
	return strings.HasSuffix(name, ".sql") || name == "atlas.sum"
}

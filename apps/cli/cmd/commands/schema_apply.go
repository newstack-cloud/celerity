package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/celerity/apps/cli/internal/deployconfig"
	"github.com/newstack-cloud/celerity/apps/cli/internal/deploydb"
	"github.com/newstack-cloud/celerity/apps/cli/internal/engine"
	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"github.com/newstack-cloud/deploy-cli-sdk/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/term"
)

func setupSchemaApplyCommand(schemaCmd *cobra.Command, confProvider *config.Provider) {
	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending migrations to a database",
		Long: `Apply pending migration files from the resource's migrationsPath to a database,
in timestamp order, skipping migrations already recorded in the
atlas_schema_revisions table.

Choose the target database with one of:
  --url            an explicit connection string (local or any reachable database)
  --instance-name  a deployed blueprint instance; the connection is resolved from
                   its deployed state and the credential is fetched from the
                   provider's secret store (AWS today)

Examples:
  celerity schema apply --url postgres://user:pass@localhost:5432/orders?sslmode=disable
  celerity schema apply --instance-name production-orders`,
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			instanceName, _ := cmd.Flags().GetString("instance-name")
			via, _ := cmd.Flags().GetString("via")

			if err := validateVia(via); err != nil {
				return err
			}
			if url == "" && instanceName == "" {
				return fmt.Errorf("provide a target database with --url or --instance-name")
			}
			if url != "" && instanceName != "" {
				return fmt.Errorf("--url and --instance-name are mutually exclusive")
			}

			resource, err := resolveDatabaseResource(cmd)
			if err != nil {
				return err
			}
			if resource.MigrationsPath == "" {
				return fmt.Errorf("resource %q has no migrationsPath configured", resource.ResourceName)
			}

			target := "the target database"
			if url == "" {
				// A deployed/live database must be migrated from a committed,
				// verified directory, so a valid atlas.sum is required rather
				// than auto-generated.
				if err := sqlschema.ValidateMigrationDir(resource.MigrationsPath); err != nil {
					return err
				}
				deployed, err := resolveDeployedInstance(cmd.Context(), confProvider, instanceName)
				if err != nil {
					return err
				}

				// This runs before migrating rather than after. Migrations run as
				// the owner and do not need the application user, but a database
				// that was restored or replaced has an application credential
				// authenticating as nobody, and this is the command an operator
				// reaches for.
				if via != viaDirect {
					if err := provisionAppUser(
						cmd.Context(), cmd.OutOrStdout(), deployed.State,
						deployed.DeployTarget, resource.ResourceName, via,
					); err != nil {
						return err
					}
				}

				// Migrations move to the database rather than the other way
				// round, when there is a runner to move them to. A private
				// database cannot be reached from here at all, so this is not an
				// optimisation. The direct path below only works for a database
				// the operator can actually open a connection to.
				maxRisk := maxRiskFromFlags(cmd)
				handled, err := applyMigrationsViaRunner(
					cmd.Context(), cmd.OutOrStdout(), deployed.State, deployed.DeployTarget,
					resource, via, maxRisk, applyConfirmation(cmd),
				)
				if err != nil {
					return err
				}
				if handled {
					return nil
				}

				resolved, err := deployed.connection(cmd.Context(), resource)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Applying to %s\n", resolved.Target)
				target = resolved.Target
				url = resolved.ConnectionString
			}

			applier, err := sqlschema.NewApplier(resource.Engine, url, zap.NewNop())
			if err != nil {
				return err
			}
			defer applier.Close()

			out := cmd.OutOrStdout()

			// The database state is guarded before applying, because drift means
			// the live database is not in the state the pending migrations assume
			// and even correct migrations can then do the wrong thing. History
			// drift (a revision applied out-of-band or a migration deleted from
			// the directory) is always checked. Structural drift additionally
			// needs a dev database to reconstruct the expected state, which is
			// either a throwaway container or --dev-url, falling back to
			// history-only when neither is available. --allow-drift skips the
			// whole check.
			if allowDrift, _ := cmd.Flags().GetBool("allow-drift"); !allowDrift {
				devURL, _ := cmd.Flags().GetString("dev-url")
				if devURL == "" && (resource.Engine == "" || resource.Engine == "postgres") {
					fmt.Fprintln(out, "Checking for drift before applying...")
					u, cleanup, provErr := provisionDevPostgres(cmd.Context(), zap.NewNop())
					if provErr != nil {
						fmt.Fprintf(out,
							"Note: structural drift check skipped (no dev database: %v); running history-only. Pass --dev-url for the full check.\n",
							provErr)
					} else {
						defer cleanup()
						devURL = u
					}
				}

				report, err := sqlschema.DetectDrift(cmd.Context(), applier, devURL, resource.MigrationsPath, resource.Engine)
				if err != nil {
					return err
				}
				if report.HasDrift() {
					report.Resource = resource.ResourceName
					report.Target = target
					report.Contracts, _ = evaluateResourceContracts(cmd, resource, report.StructuralTables)
					report.Render(out)
					return fmt.Errorf(
						"drift detected for %s; run `celerity schema drift` to inspect it, then reconcile or re-run with --allow-drift once understood",
						resource.ResourceName)
				}
			}

			maxRisk := maxRiskFromFlags(cmd)

			pending, err := applier.PendingMigrations(cmd.Context(), resource.MigrationsPath)
			if err != nil {
				return err
			}
			if len(pending) == 0 {
				fmt.Fprintf(out, "No pending migrations for %s.\n", resource.ResourceName)
				return nil
			}

			fmt.Fprintf(out, "%d pending migration(s) for %s → %s:\n", len(pending), resource.ResourceName, target)
			for _, p := range pending {
				fmt.Fprintf(out, "  - %s  [risk: %s]\n", p.Name, p.Risk)
			}

			// Applying to a named deployed instance is confirmed interactively
			// unless --auto-approve is passed (e.g. in CI). Explicit --url targets
			// apply directly.
			if instanceName != "" {
				autoApprove, _ := cmd.Flags().GetBool("auto-approve")
				if !autoApprove {
					if err := confirmApply(cmd, out); err != nil {
						return err
					}
				}
			}

			if err := applier.ApplyMigrationDir(cmd.Context(), resource.MigrationsPath, maxRisk); err != nil {
				return err
			}
			fmt.Fprintf(out, "Applied pending migrations for %s.\n", resource.ResourceName)
			return nil
		},
	}

	applyCmd.Flags().String("url", "", "Connection string of the target database.")
	applyCmd.Flags().String("instance-name", "", "Deployed blueprint instance whose database to migrate.")
	applyCmd.Flags().Bool("allow-risky", false, "Apply data-dependent (risky) migrations (e.g. NOT NULL, unique, foreign key).")
	applyCmd.Flags().Bool("allow-breaking", false, "Apply breaking (destructive) migrations (e.g. drop table/column). Implies --allow-risky.")
	applyCmd.Flags().Bool("allow-drift", false, "Skip the pre-apply drift check.")
	applyCmd.Flags().String("dev-url", "", "Empty dev database for the structural drift check (default: start a throwaway container).")
	applyCmd.Flags().BoolP("auto-approve", "y", false, "Skip the interactive confirmation when applying to a deployed instance.")
	applyCmd.Flags().String("via", viaAuto,
		"How to reach the database: auto (use the deployed maintenance runner when there is one), runner (require it), direct (connect from here).")
	schemaCmd.AddCommand(applyCmd)
}

// confirmApply prompts for confirmation before applying migrations to a deployed
// database. It fails when there is no terminal to prompt on (e.g. CI), directing
// the user to --auto-approve.
func confirmApply(cmd *cobra.Command, out io.Writer) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("refusing to apply to a deployed instance without confirmation; re-run with --auto-approve")
	}
	fmt.Fprint(out, "Proceed? [y/N]: ")
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("apply aborted")
	}
}

// deployedInstance is a resolved deployment, holding its state and the deploy
// target that determines how anything provider-specific is done against it.
//
// It is resolved once and reused, because both halves of an apply against a
// deployed database need it: provisioning the application user through the
// maintenance runner, and building the migration connection.
type deployedInstance struct {
	State        *state.InstanceState
	DeployTarget string
}

// resolveDeployedInstance reads the deploy target from config and fetches the
// named instance's state from the deploy engine.
func resolveDeployedInstance(
	ctx context.Context,
	confProvider *config.Provider,
	instanceName string,
) (*deployedInstance, error) {
	appConfigFile, _ := confProvider.GetString("appDeployConfigFile")
	if appConfigFile == "" {
		appConfigFile = "app.deploy.jsonc"
	}
	appCfg, err := deployconfig.ReadAppDeployConfig(appConfigFile)
	if err != nil {
		return nil, fmt.Errorf("reading deploy config: %w", err)
	}

	engineClient, err := engine.Create(confProvider, zap.NewNop())
	if err != nil {
		return nil, fmt.Errorf("connecting to deploy engine: %w", err)
	}

	instanceState, err := getInstanceByName(ctx, engineClient, instanceName)
	if err != nil {
		return nil, err
	}

	return &deployedInstance{State: instanceState, DeployTarget: appCfg.DeployTarget.Name}, nil
}

// resolveDeployedConnection resolves a migration connection for a deployed
// instance in one step, for the read-only commands that have no reason to
// provision anything.
func resolveDeployedConnection(
	ctx context.Context,
	confProvider *config.Provider,
	resource sqlschema.DatabaseResource,
	instanceName string,
) (*deploydb.ResolvedConnection, error) {
	deployed, err := resolveDeployedInstance(ctx, confProvider, instanceName)
	if err != nil {
		return nil, err
	}
	return deployed.connection(ctx, resource)
}

// connection derives a migration connection (credentials included) from the
// instance's state, using the provider-specific resolver for its deploy target.
func (d *deployedInstance) connection(
	ctx context.Context,
	resource sqlschema.DatabaseResource,
) (*deploydb.ResolvedConnection, error) {
	resolver, err := deploydb.ResolverFor(d.DeployTarget)
	if err != nil {
		return nil, err
	}
	return resolver.Resolve(ctx, d.State, deploydb.ResolveInput{
		SQLDatabaseName:  resource.ResourceName,
		Engine:           resource.Engine,
		FallbackDatabase: resource.Name,
	})
}

// getInstanceByName finds a deployed blueprint instance by its exact name and
// returns its full state.
func getInstanceByName(ctx context.Context, engineClient engine.DeployEngine, name string) (*state.InstanceState, error) {
	list, err := engineClient.ListBlueprintInstances(ctx, state.ListInstancesParams{Search: name})
	if err != nil {
		return nil, fmt.Errorf("listing deployed instances: %w", err)
	}
	for _, summary := range list.Instances {
		if summary.InstanceName == name {
			return engineClient.GetBlueprintInstance(ctx, summary.InstanceID)
		}
	}
	return nil, fmt.Errorf("no deployed instance named %q found", name)
}

// maxRiskFromFlags reads the highest migration risk the operator authorised.
// Breaking implies risky, which is why it is checked second.
func maxRiskFromFlags(cmd *cobra.Command) sqlschema.RiskLevel {
	maxRisk := sqlschema.RiskSafe
	if allowRisky, _ := cmd.Flags().GetBool("allow-risky"); allowRisky {
		maxRisk = sqlschema.RiskRisky
	}
	if allowBreaking, _ := cmd.Flags().GetBool("allow-breaking"); allowBreaking {
		maxRisk = sqlschema.RiskBreaking
	}
	return maxRisk
}

// applyConfirmation returns the interactive confirmation to run before applying
// to a deployed instance, or nil when none is needed.
//
// Applying through the runner reaches a deployed database just as the direct
// path does, so it asks the same question. --auto-approve skips it for CI.
func applyConfirmation(cmd *cobra.Command) func() error {
	if autoApprove, _ := cmd.Flags().GetBool("auto-approve"); autoApprove {
		return nil
	}
	return func() error { return confirmApply(cmd, cmd.OutOrStdout()) }
}

package commands

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/newstack-cloud/celerity/apps/cli/internal/sqlschema"
	"go.uber.org/zap"
)

const (
	devPostgresImage     = "postgres:17-alpine"
	devPostgresContainer = "celerity-schema-devdb"
	devPostgresUser      = "celerity"
	devPostgresPassword  = "celerity"
	devPostgresDatabase  = "celerity_devdb"
)

// Starts a throwaway PostgreSQL container for the Atlas migration planner to
// replay the migration directory into, and returns its connection string plus a
// cleanup function that removes the container.
//
// Used by the commands that need a dev database when no --dev-url is supplied.
func provisionDevPostgres(ctx context.Context, logger *zap.Logger) (string, func(), error) {
	mgr, err := docker.NewRuntimeContainer(logger)
	if err != nil {
		return "", nil, fmt.Errorf("creating docker client: %w", err)
	}
	if err := mgr.CheckAvailability(ctx); err != nil {
		return "", nil, fmt.Errorf("docker is required for `schema diff` (or pass --dev-url): %w", err)
	}

	// Clean up any container left behind by a previous interrupted run.
	_ = mgr.CleanupStale(ctx, devPostgresContainer)

	// EnsureImage closes the progress channel itself (defer close), so it is
	// only drained here. Closing it again would panic.
	progress := make(chan docker.ImagePullProgress)
	go func() {
		for range progress { // drain
		}
	}()
	if err := mgr.EnsureImage(ctx, devPostgresImage, progress); err != nil {
		return "", nil, fmt.Errorf("pulling %s: %w", devPostgresImage, err)
	}

	hostPort, err := freePort()
	if err != nil {
		return "", nil, err
	}

	id, err := mgr.CreateAndStart(ctx, &docker.ContainerConfig{
		Image:         devPostgresImage,
		ContainerName: devPostgresContainer,
		HostPort:      hostPort,
		ContainerPort: "5432",
		EnvVars: map[string]string{
			"POSTGRES_USER":     devPostgresUser,
			"POSTGRES_PASSWORD": devPostgresPassword,
			"POSTGRES_DB":       devPostgresDatabase,
		},
	})
	if err != nil {
		return "", nil, err
	}

	cleanup := func() { _ = mgr.Stop(context.WithoutCancel(ctx), id) }

	connStr := sqlschema.FormatConnectionString(
		"localhost", hostPort, devPostgresUser, devPostgresPassword, devPostgresDatabase,
	)
	if err := waitForPostgres(ctx, connStr, 30*time.Second); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("dev database did not become ready: %w", err)
	}

	return connStr, cleanup, nil
}

// Polls until the database accepts connections or the timeout elapses.
func waitForPostgres(ctx context.Context, connStr string, timeout time.Duration) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	deadline := time.Now().Add(timeout)
	for {
		if err := db.PingContext(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("finding a free port: %w", err)
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}
	return port, nil
}

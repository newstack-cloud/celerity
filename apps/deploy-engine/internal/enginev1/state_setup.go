package enginev1

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/afero"
	"github.com/two-hundred/celerity/apps/deploy-engine/core"
	"github.com/two-hundred/celerity/libs/blueprint-state/memfile"
	"github.com/two-hundred/celerity/libs/blueprint-state/postgres"
	bpcore "github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

const (
	memfileStorageEngine  = "memfile"
	postgresStorageEngine = "postgres"
)

func loadStateContainer(
	ctx context.Context,
	fileSystem afero.Fs,
	logger bpcore.Logger,
	stateConfig *core.StateConfig,
) (state.Container, error) {
	if stateConfig.StorageEngine == memfileStorageEngine {
		return memfile.LoadStateContainer(
			stateConfig.MemFileStateDir,
			fileSystem,
			logger,
			memfile.WithMaxGuideFileSize(
				stateConfig.MemFileMaxGuideFileSize,
			),
		)
	}

	if stateConfig.StorageEngine == postgresStorageEngine {
		pool, err := createPostgresConnPool(ctx, stateConfig)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to create postgres connection pool: %w",
				err,
			)
		}

		return postgres.LoadStateContainer(
			ctx,
			pool,
			logger,
		)
	}

	return nil, fmt.Errorf(
		"unsupported %q storage engine provided, "+
			"only the \"memfile\" and \"postgres\" engines are supported"+
			" for this version of the deploy engine",
		stateConfig.StorageEngine,
	)
}

func createPostgresConnPool(
	ctx context.Context,
	stateConfig *core.StateConfig,
) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, buildPostgresDatabaseURL(stateConfig))
}

func buildPostgresDatabaseURL(stateConfig *core.StateConfig) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d&pool_max_conn_lifetime=%d",
		stateConfig.PostgresUser,
		stateConfig.PostgresPassword,
		stateConfig.PostgresHost,
		stateConfig.PostgresPort,
		stateConfig.PostgresDatabase,
		stateConfig.PostgresSSLMode,
		stateConfig.PostgresPoolMaxConns,
		stateConfig.PostgresPoolMaxConnLifetime,
	)
}

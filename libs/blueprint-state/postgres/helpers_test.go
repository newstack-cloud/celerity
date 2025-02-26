package postgres

import (
	"fmt"
	"os"
)

func buildTestDatabaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&pool_max_conns=%s&pool_max_conn_lifetime=%s",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
		os.Getenv("POSTGRES_POOL_MAX_CONNS"),
		os.Getenv("POSTGRES_POOL_MAX_CONN_LIFETIME"),
	)
}

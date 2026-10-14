package sqlschema

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ariga.io/atlas/sql/migrate"
)

// The migration-history table Celerity owns. It matches the
// Atlas default name so teams that separately use Atlas tooling against the same
// migration directory see a consistent history table.
const revisionsTable = "atlas_schema_revisions"

// On Postgres the history table lives in its own schema, as Atlas does by
// default, so that it sits outside the schema the YAML models. Nothing scoped to
// the managed schema (declarative diffs, `schema show`, exports) can reach it,
// let alone drop it.
const (
	pgRevisionsSchema    = "atlas_schema_revisions"
	pgRevisionsQualified = `"` + pgRevisionsSchema + `"."` + revisionsTable + `"`
	// Where the table lived before it was moved into its own schema. Databases
	// migrated by an earlier CLI still have it here; ensureTable adopts those
	// rows on first use.
	pgLegacyRevisionsQualified = `"` + postgresSchemaName + `"."` + revisionsTable + `"`
)

// Implements migrate.RevisionReadWriter over a Postgres connection.
// Atlas has no public concrete revision store, so we own this table, which keeps
// the table name (atlas_schema_revisions) under our control as the Gen 3 spec
// requires. The Revision struct's Applied/Total/PartialHashes fields carry
// statement-level progress so a non-transactional migration can resume safely.
type pgRevisions struct {
	db *sql.DB
}

func newPGRevisions(ctx context.Context, db *sql.DB) (*pgRevisions, error) {
	p := &pgRevisions{db: db}
	if err := p.ensureTable(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *pgRevisions) ensureTable(ctx context.Context) error {
	if _, err := p.db.ExecContext(ctx,
		fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %q", pgRevisionsSchema),
	); err != nil {
		return fmt.Errorf("creating %s schema: %w", pgRevisionsSchema, err)
	}

	_, err := p.db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version varchar NOT NULL PRIMARY KEY,
		description text NOT NULL,
		type bigint NOT NULL DEFAULT 0,
		applied bigint NOT NULL DEFAULT 0,
		total bigint NOT NULL DEFAULT 0,
		executed_at timestamptz NOT NULL,
		execution_time bigint NOT NULL DEFAULT 0,
		error text NOT NULL DEFAULT '',
		error_stmt text NOT NULL DEFAULT '',
		hash text NOT NULL DEFAULT '',
		partial_hashes jsonb,
		operator_version text NOT NULL DEFAULT ''
	)`, pgRevisionsQualified))
	if err != nil {
		return fmt.Errorf("creating %s table: %w", pgRevisionsQualified, err)
	}

	return p.adoptLegacyRevisions(ctx)
}

// Moves history written by an earlier CLI, which kept the table in the managed
// schema, into the dedicated schema. It runs in one transaction and copies before
// dropping, so an interrupted move leaves the history readable in the old
// location rather than lost.
//
// Rows already in the new table win, because a database that has been migrated
// since the move is authoritative and the legacy table is stale.
func (p *pgRevisions) adoptLegacyRevisions(ctx context.Context) error {
	var exists bool
	err := p.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = $1 AND table_name = $2
	)`, postgresSchemaName, revisionsTable).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking for a legacy %s table: %w", revisionsTable, err)
	}
	if !exists {
		return nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting the migration-history move: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (%s) SELECT %s FROM %s ON CONFLICT (version) DO NOTHING",
		pgRevisionsQualified, revisionColumns, revisionColumns, pgLegacyRevisionsQualified,
	)); err != nil {
		return fmt.Errorf("copying migration history out of %s: %w", postgresSchemaName, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", pgLegacyRevisionsQualified)); err != nil {
		return fmt.Errorf("dropping the legacy %s table: %w", revisionsTable, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing the migration-history move: %w", err)
	}
	return nil
}

func (p *pgRevisions) Ident() *migrate.TableIdent {
	return &migrate.TableIdent{Name: revisionsTable, Schema: pgRevisionsSchema}
}

func (p *pgRevisions) ReadRevisions(ctx context.Context) ([]*migrate.Revision, error) {
	rows, err := p.db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s ORDER BY version", revisionColumns, pgRevisionsQualified))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []*migrate.Revision
	for rows.Next() {
		r, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, r)
	}
	return revisions, rows.Err()
}

func (p *pgRevisions) ReadRevision(ctx context.Context, version string) (*migrate.Revision, error) {
	row := p.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE version = $1", revisionColumns, pgRevisionsQualified), version)
	r, err := scanRevision(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, migrate.ErrRevisionNotExist
	}
	return r, err
}

func (p *pgRevisions) WriteRevision(ctx context.Context, r *migrate.Revision) error {
	partialHashes, err := json.Marshal(r.PartialHashes)
	if err != nil {
		return fmt.Errorf("marshalling partial hashes: %w", err)
	}
	_, err = p.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s
		(version, description, type, applied, total, executed_at, execution_time, error, error_stmt, hash, partial_hashes, operator_version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (version) DO UPDATE SET
			description=$2, type=$3, applied=$4, total=$5, executed_at=$6, execution_time=$7,
			error=$8, error_stmt=$9, hash=$10, partial_hashes=$11, operator_version=$12`, pgRevisionsQualified),
		r.Version, r.Description, int64(r.Type), r.Applied, r.Total, r.ExecutedAt,
		int64(r.ExecutionTime), r.Error, r.ErrorStmt, r.Hash, partialHashes, r.OperatorVersion)
	if err != nil {
		return fmt.Errorf("writing revision %s: %w", r.Version, err)
	}
	return nil
}

func (p *pgRevisions) DeleteRevision(ctx context.Context, version string) error {
	_, err := p.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE version = $1", pgRevisionsQualified), version)
	return err
}

const revisionColumns = "version, description, type, applied, total, executed_at, execution_time, error, error_stmt, hash, partial_hashes, operator_version"

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRevision(s rowScanner) (*migrate.Revision, error) {
	var (
		r             migrate.Revision
		revType       int64
		executionTime int64
		partialHashes []byte
	)
	if err := s.Scan(
		&r.Version, &r.Description, &revType, &r.Applied, &r.Total, &r.ExecutedAt,
		&executionTime, &r.Error, &r.ErrorStmt, &r.Hash, &partialHashes, &r.OperatorVersion,
	); err != nil {
		return nil, err
	}
	r.Type = migrate.RevisionType(revType)
	r.ExecutionTime = time.Duration(executionTime)
	if len(partialHashes) > 0 {
		if err := json.Unmarshal(partialHashes, &r.PartialHashes); err != nil {
			return nil, fmt.Errorf("unmarshalling partial hashes for %s: %w", r.Version, err)
		}
	}
	return &r, nil
}

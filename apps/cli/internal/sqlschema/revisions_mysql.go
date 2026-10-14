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

// mysqlRevisions implements migrate.RevisionReadWriter over a MySQL connection,
// backing the same atlas_schema_revisions table as the Postgres store but with
// MySQL-compatible DDL and an upsert.
type mysqlRevisions struct {
	db     *sql.DB
	schema string
}

func newMySQLRevisions(ctx context.Context, db *sql.DB) (*mysqlRevisions, error) {
	schema, err := mysqlDialect{}.SchemaName(ctx, db)
	if err != nil {
		return nil, err
	}
	p := &mysqlRevisions{db: db, schema: schema}
	if err := p.ensureTable(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *mysqlRevisions) ensureTable(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version varchar(255) NOT NULL PRIMARY KEY,
		description text NOT NULL,
		type bigint NOT NULL DEFAULT 0,
		applied bigint NOT NULL DEFAULT 0,
		total bigint NOT NULL DEFAULT 0,
		executed_at datetime(6) NOT NULL,
		execution_time bigint NOT NULL DEFAULT 0,
		error text,
		error_stmt text,
		hash varchar(255) NOT NULL DEFAULT '',
		partial_hashes json,
		operator_version varchar(255) NOT NULL DEFAULT ''
	)`, revisionsTable))
	if err != nil {
		return fmt.Errorf("creating %s table: %w", revisionsTable, err)
	}
	return nil
}

// For MySQL the revision table's schema is the connected database.
func (p *mysqlRevisions) Ident() *migrate.TableIdent {
	return &migrate.TableIdent{Name: revisionsTable, Schema: p.schema}
}

func (p *mysqlRevisions) ReadRevisions(ctx context.Context) ([]*migrate.Revision, error) {
	rows, err := p.db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s ORDER BY version", revisionColumns, revisionsTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []*migrate.Revision
	for rows.Next() {
		r, err := scanMySQLRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, r)
	}
	return revisions, rows.Err()
}

func (p *mysqlRevisions) ReadRevision(ctx context.Context, version string) (*migrate.Revision, error) {
	row := p.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE version = ?", revisionColumns, revisionsTable), version)
	r, err := scanMySQLRevision(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, migrate.ErrRevisionNotExist
	}
	return r, err
}

func (p *mysqlRevisions) WriteRevision(ctx context.Context, r *migrate.Revision) error {
	partialHashes, err := json.Marshal(r.PartialHashes)
	if err != nil {
		return fmt.Errorf("marshalling partial hashes: %w", err)
	}
	_, err = p.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s
		(version, description, type, applied, total, executed_at, execution_time, error, error_stmt, hash, partial_hashes, operator_version)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
			description=VALUES(description), type=VALUES(type), applied=VALUES(applied), total=VALUES(total),
			executed_at=VALUES(executed_at), execution_time=VALUES(execution_time), error=VALUES(error),
			error_stmt=VALUES(error_stmt), hash=VALUES(hash), partial_hashes=VALUES(partial_hashes),
			operator_version=VALUES(operator_version)`, revisionsTable),
		r.Version, r.Description, int64(r.Type), r.Applied, r.Total, r.ExecutedAt,
		int64(r.ExecutionTime), r.Error, r.ErrorStmt, r.Hash, partialHashes, r.OperatorVersion)
	if err != nil {
		return fmt.Errorf("writing revision %s: %w", r.Version, err)
	}
	return nil
}

func (p *mysqlRevisions) DeleteRevision(ctx context.Context, version string) error {
	_, err := p.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE version = ?", revisionsTable), version)
	return err
}

func scanMySQLRevision(s rowScanner) (*migrate.Revision, error) {
	var (
		r                migrate.Revision
		revType          int64
		executionTime    int64
		errText, errStmt sql.NullString
		partialHashes    []byte
	)
	if err := s.Scan(
		&r.Version, &r.Description, &revType, &r.Applied, &r.Total, &r.ExecutedAt,
		&executionTime, &errText, &errStmt, &r.Hash, &partialHashes, &r.OperatorVersion,
	); err != nil {
		return nil, err
	}
	r.Type = migrate.RevisionType(revType)
	r.ExecutionTime = time.Duration(executionTime)
	r.Error = errText.String
	r.ErrorStmt = errStmt.String
	if len(partialHashes) > 0 {
		if err := json.Unmarshal(partialHashes, &r.PartialHashes); err != nil {
			return nil, fmt.Errorf("unmarshalling partial hashes for %s: %w", r.Version, err)
		}
	}
	return &r, nil
}

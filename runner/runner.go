package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/flowgrate/core/adapters/postgres"
	"github.com/flowgrate/core/manifest"
)

const createHistoryTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    id         SERIAL PRIMARY KEY,
    migration  VARCHAR(255) NOT NULL UNIQUE,
    batch      INTEGER      NOT NULL,
    applied_at TIMESTAMP    NOT NULL DEFAULT NOW()
);`

type Runner struct {
	conn *pgx.Conn
}

// AppliedMigration holds a record from schema_migrations.
type AppliedMigration struct {
	Migration string
	Batch     int
	AppliedAt time.Time
}

func New(ctx context.Context, dsn string) (*Runner, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &Runner{conn: conn}, nil
}

func (r *Runner) Close(ctx context.Context) {
	r.conn.Close(ctx)
}

func (r *Runner) Init(ctx context.Context) error {
	_, err := r.conn.Exec(ctx, createHistoryTable)
	return err
}

func (r *Runner) IsApplied(ctx context.Context, migration string) (bool, error) {
	var exists bool
	err := r.conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE migration = $1)", migration,
	).Scan(&exists)
	return exists, err
}

func (r *Runner) NextBatch(ctx context.Context) (int, error) {
	var batch int
	err := r.conn.QueryRow(ctx,
		"SELECT COALESCE(MAX(batch), 0) + 1 FROM schema_migrations",
	).Scan(&batch)
	return batch, err
}

func (r *Runner) Up(ctx context.Context, m manifest.Migration, batch int) error {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, op := range m.Up {
		sqls, err := postgres.Compile(op)
		if err != nil {
			return fmt.Errorf("compile %s: %w", op.Action, err)
		}
		for _, sql := range sqls {
			if _, err := tx.Exec(ctx, sql); err != nil {
				return fmt.Errorf("execute sql:\n%s\nerror: %w", sql, err)
			}
		}
	}

	_, err = tx.Exec(ctx,
		"INSERT INTO schema_migrations (migration, batch) VALUES ($1, $2)",
		m.Migration, batch,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Runner) Down(ctx context.Context, m manifest.Migration) error {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, op := range m.Down {
		sqls, err := postgres.Compile(op)
		if err != nil {
			return fmt.Errorf("compile %s: %w", op.Action, err)
		}
		for _, sql := range sqls {
			if _, err := tx.Exec(ctx, sql); err != nil {
				return fmt.Errorf("execute sql:\n%s\nerror: %w", sql, err)
			}
		}
	}

	_, err = tx.Exec(ctx,
		"DELETE FROM schema_migrations WHERE migration = $1", m.Migration,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ListApplied returns all applied migrations ordered by application order.
func (r *Runner) ListApplied(ctx context.Context) ([]AppliedMigration, error) {
	rows, err := r.conn.Query(ctx,
		"SELECT migration, batch, applied_at FROM schema_migrations ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AppliedMigration
	for rows.Next() {
		var m AppliedMigration
		if err := rows.Scan(&m.Migration, &m.Batch, &m.AppliedAt); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// DropAllTables drops every table in the public schema (CASCADE).
// Used by the fresh command.
func (r *Runner) DropAllTables(ctx context.Context) error {
	_, err := r.conn.Exec(ctx, `
		DO $$ DECLARE
			r RECORD;
		BEGIN
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
				EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
			END LOOP;
		END $$;
	`)
	return err
}

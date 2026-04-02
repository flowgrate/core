package runner

import (
	"context"
	"fmt"

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

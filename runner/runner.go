package runner

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	// Register drivers.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/flowgrate/core/adapters"
	"github.com/flowgrate/core/manifest"
)

// Runner executes migrations against a database.
type Runner struct {
	db      *sql.DB
	adapter adapters.Adapter
}

// AppliedMigration holds a record from schema_migrations.
type AppliedMigration struct {
	Migration string
	Batch     int
	AppliedAt time.Time
}

// New opens a database connection based on the DSN scheme and returns a Runner.
func New(ctx context.Context, dsn string) (*Runner, error) {
	adapter, err := adapters.New(dsn)
	if err != nil {
		return nil, err
	}
	formatted, err := adapter.FormatDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("format dsn: %w", err)
	}
	db, err := sql.Open(adapter.DriverName(), formatted)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &Runner{db: db, adapter: adapter}, nil
}

func (r *Runner) Close(_ context.Context) {
	r.db.Close()
}

// Init creates the schema_migrations table if it doesn't exist.
func (r *Runner) Init(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, r.adapter.CreateHistorySQL())
	return err
}

func (r *Runner) IsApplied(ctx context.Context, migration string) (bool, error) {
	q := fmt.Sprintf(
		"SELECT COUNT(*) FROM schema_migrations WHERE migration = %s",
		r.adapter.Placeholder(1),
	)
	var count int
	err := r.db.QueryRowContext(ctx, q, migration).Scan(&count)
	return count > 0, err
}

func (r *Runner) NextBatch(ctx context.Context) (int, error) {
	var batch int
	err := r.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(batch), 0) + 1 FROM schema_migrations",
	).Scan(&batch)
	return batch, err
}

func (r *Runner) Up(ctx context.Context, m manifest.Migration, batch int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, op := range m.Up {
		sqls, err := r.adapter.Compile(op)
		if err != nil {
			return fmt.Errorf("compile %s: %w", op.Action, err)
		}
		for _, s := range sqls {
			if _, err := tx.ExecContext(ctx, s); err != nil {
				return fmt.Errorf("execute sql:\n%s\nerror: %w", s, err)
			}
		}
	}

	q := fmt.Sprintf(
		"INSERT INTO schema_migrations (migration, batch) VALUES (%s, %s)",
		r.adapter.Placeholder(1), r.adapter.Placeholder(2),
	)
	if _, err := tx.ExecContext(ctx, q, m.Migration, batch); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Runner) Down(ctx context.Context, m manifest.Migration) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, op := range m.Down {
		sqls, err := r.adapter.Compile(op)
		if err != nil {
			return fmt.Errorf("compile %s: %w", op.Action, err)
		}
		for _, s := range sqls {
			if _, err := tx.ExecContext(ctx, s); err != nil {
				return fmt.Errorf("execute sql:\n%s\nerror: %w", s, err)
			}
		}
	}

	q := fmt.Sprintf(
		"DELETE FROM schema_migrations WHERE migration = %s",
		r.adapter.Placeholder(1),
	)
	if _, err := tx.ExecContext(ctx, q, m.Migration); err != nil {
		return err
	}
	return tx.Commit()
}

// ListApplied returns all applied migrations ordered by application order.
func (r *Runner) ListApplied(ctx context.Context) ([]AppliedMigration, error) {
	rows, err := r.db.QueryContext(ctx,
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

// IsFresh returns true if the schema_migrations table doesn't exist or has no rows.
func (r *Runner) IsFresh(ctx context.Context) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM schema_migrations",
	).Scan(&count)
	if err != nil {
		if isMissingTableError(err) {
			return true, nil
		}
		return false, err
	}
	return count == 0, nil
}

// DropAllTables drops every user table except schema_migrations.
func (r *Runner) DropAllTables(ctx context.Context) error {
	for _, s := range r.adapter.PreDropSQL() {
		if _, err := r.db.ExecContext(ctx, s); err != nil {
			return err
		}
	}

	rows, err := r.db.QueryContext(ctx, r.adapter.ListTablesSQL())
	if err != nil {
		return err
	}
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return err
		}
		tables = append(tables, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, t := range tables {
		if _, err := r.db.ExecContext(ctx, r.adapter.DropTableSQL(t)); err != nil {
			return err
		}
	}

	for _, s := range r.adapter.PostDropSQL() {
		if _, err := r.db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// isMissingTableError returns true when the error means the table doesn't exist yet.
func isMissingTableError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "no such table")
}

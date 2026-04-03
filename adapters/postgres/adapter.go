package postgres

import (
	"fmt"

	"github.com/flowgrate/core/manifest"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) DriverName() string { return "pgx" }

func (a *Adapter) FormatDSN(rawURL string) (string, error) {
	// pgx/stdlib accepts postgres:// URLs directly.
	return rawURL, nil
}

func (a *Adapter) Compile(op manifest.Operation) ([]string, error) {
	return Compile(op)
}

func (a *Adapter) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (a *Adapter) CreateHistorySQL() string {
	return `CREATE TABLE IF NOT EXISTS schema_migrations (
    id         SERIAL PRIMARY KEY,
    migration  VARCHAR(255) NOT NULL UNIQUE,
    batch      INTEGER      NOT NULL,
    applied_at TIMESTAMP    NOT NULL DEFAULT NOW()
);`
}

func (a *Adapter) ListTablesSQL() string {
	return `SELECT tablename FROM pg_tables
	        WHERE schemaname = 'public' AND tablename != 'schema_migrations'`
}

func (a *Adapter) DropTableSQL(table string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", table)
}

func (a *Adapter) PreDropSQL() []string  { return nil }
func (a *Adapter) PostDropSQL() []string { return nil }

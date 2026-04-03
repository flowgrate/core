package adapters

import (
	"fmt"
	"net/url"

	"github.com/flowgrate/core/adapters/mysql"
	"github.com/flowgrate/core/adapters/postgres"
	"github.com/flowgrate/core/adapters/sqlite"
	"github.com/flowgrate/core/manifest"
)

// Adapter handles database-specific SQL generation and connection string formatting.
type Adapter interface {
	DriverName() string
	FormatDSN(rawURL string) (string, error)
	Compile(op manifest.Operation) ([]string, error)
	Placeholder(n int) string // "$N" for postgres, "?" for mysql/sqlite
	CreateHistorySQL() string
	ListTablesSQL() string
	DropTableSQL(table string) string
	PreDropSQL() []string  // run before dropping tables (e.g. disable FK checks)
	PostDropSQL() []string // run after dropping tables
}

// New returns the Adapter matching the DSN scheme.
func New(rawURL string) (Adapter, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		return postgres.NewAdapter(), nil
	case "mysql", "mariadb":
		return mysql.NewAdapter(), nil
	case "sqlite", "sqlite3":
		return sqlite.NewAdapter(), nil
	default:
		return nil, fmt.Errorf("unsupported driver %q (supported: postgres, mysql, sqlite)", u.Scheme)
	}
}

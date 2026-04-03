package sqlite

import (
	"fmt"
	"net/url"

	"github.com/flowgrate/core/manifest"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) DriverName() string { return "sqlite" }

// FormatDSN extracts the file path from a sqlite:// URL.
func (a *Adapter) FormatDSN(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	// sqlite:///abs/path  → /abs/path
	// sqlite://./rel      → ./rel
	path := u.Path
	if u.Host != "" {
		path = u.Host + u.Path
	}
	if path == "" {
		return "", fmt.Errorf("sqlite DSN missing file path: %s", rawURL)
	}
	return path, nil
}

func (a *Adapter) Compile(op manifest.Operation) ([]string, error) {
	return Compile(op)
}

func (a *Adapter) Placeholder(_ int) string { return "?" }

func (a *Adapter) CreateHistorySQL() string {
	return `CREATE TABLE IF NOT EXISTS schema_migrations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    migration  TEXT    NOT NULL UNIQUE,
    batch      INTEGER NOT NULL,
    applied_at TEXT    NOT NULL DEFAULT (datetime('now'))
);`
}

func (a *Adapter) ListTablesSQL() string {
	return `SELECT name FROM sqlite_master
	        WHERE type = 'table'
	          AND name != 'schema_migrations'
	          AND name NOT LIKE 'sqlite_%'`
}

func (a *Adapter) DropTableSQL(table string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", table)
}

// PreDropSQL enables foreign key enforcement so we must disable it before drops.
func (a *Adapter) PreDropSQL() []string {
	return []string{"PRAGMA foreign_keys = OFF;"}
}

func (a *Adapter) PostDropSQL() []string {
	return []string{"PRAGMA foreign_keys = ON;"}
}

package squash

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const migrationTable = "schema_migrations"

// SchemaState can dump and load a database schema.
type SchemaState interface {
	Dump(path string) error
	Load(path string) error
	Driver() string
}

// New returns the appropriate SchemaState for the given DSN.
func New(dsn string) (SchemaState, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		return &postgresState{u: u}, nil
	case "mysql":
		return newMySQLState(u, false), nil
	case "mariadb":
		return newMySQLState(u, true), nil
	case "sqlite", "sqlite3":
		return &sqliteState{dbPath: sqliteDBPath(u)}, nil
	default:
		return nil, fmt.Errorf("squash: unsupported driver %q (supported: postgres, mysql, mariadb, sqlite)", u.Scheme)
	}
}

// SchemaFilePath returns the canonical path for the schema dump file:
// schema/{driver}-schema.sql, stored next to the config file.
func SchemaFilePath(configDir, driver string) string {
	return filepath.Join(configDir, "schema", driver+"-schema.sql")
}

// ensureDir creates the parent directory of path if it doesn't exist.
func ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

// appendToFile appends data to the file at path.
func appendToFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// isMissingTableError returns true when pg_dump / mysqldump reports that the
// migration table doesn't exist yet (nothing has been applied).
func isMissingTableError(errMsg string) bool {
	msg := strings.ToLower(errMsg)
	return strings.Contains(msg, "no matching tables") ||
		strings.Contains(msg, migrationTable) ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist")
}

func sqliteDBPath(u *url.URL) string {
	// sqlite:///abs/path  → /abs/path
	// sqlite://./rel      → ./rel
	if u.Host == "" {
		return u.Path
	}
	return u.Host + u.Path
}

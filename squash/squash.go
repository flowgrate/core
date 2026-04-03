package squash

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
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
	default:
		return nil, fmt.Errorf("squash: unsupported driver %q (supported: postgres)", u.Scheme)
	}
}

// SchemaFilePath returns the canonical path for the schema dump file,
// stored next to the config file: schema/{driver}-schema.sql
func SchemaFilePath(configDir, driver string) string {
	return filepath.Join(configDir, "schema", driver+"-schema.sql")
}

// --- PostgreSQL ---

type postgresState struct {
	u *url.URL
}

func (s *postgresState) Driver() string { return "postgres" }

func (s *postgresState) Dump(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	env := s.env()

	// 1. Dump DDL (schema only, no data, no owner/ACL noise)
	schemaArgs := append(s.baseArgs(),
		"--schema-only",
		"--no-owner",
		"--no-acl",
	)
	out, err := s.run(env, "pg_dump", schemaArgs...)
	if err != nil {
		return fmt.Errorf("pg_dump schema: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}

	// 2. Append data-only dump of schema_migrations so fresh installs
	//    know which migrations have already been applied.
	dataArgs := append(s.baseArgs(),
		"--data-only",
		"--table="+migrationTable,
	)
	migData, err := s.run(env, "pg_dump", dataArgs...)
	if err != nil {
		// If the table doesn't exist yet (nothing applied), skip silently.
		if strings.Contains(err.Error(), "no matching tables") ||
			strings.Contains(err.Error(), migrationTable) {
			return nil
		}
		return fmt.Errorf("pg_dump migrations data: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(migData)
	return err
}

func (s *postgresState) Load(path string) error {
	args := []string{
		"--file=" + path,
		"--host=" + s.host(),
		"--port=" + s.port(),
		"--username=" + s.user(),
		"--dbname=" + s.dbname(),
		"--no-password",
	}
	cmd := exec.Command("psql", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.password())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *postgresState) baseArgs() []string {
	return []string{
		"--host=" + s.host(),
		"--port=" + s.port(),
		"--username=" + s.user(),
		"--dbname=" + s.dbname(),
		"--no-password",
	}
}

func (s *postgresState) env() []string {
	return append(os.Environ(), "PGPASSWORD="+s.password())
}

func (s *postgresState) run(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %s", name, string(ee.Stderr))
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

func (s *postgresState) host() string {
	h := s.u.Hostname()
	if h == "" {
		return "localhost"
	}
	return h
}

func (s *postgresState) port() string {
	p := s.u.Port()
	if p == "" {
		return "5432"
	}
	return p
}

func (s *postgresState) user() string {
	if s.u.User != nil {
		return s.u.User.Username()
	}
	return ""
}

func (s *postgresState) password() string {
	if s.u.User != nil {
		p, _ := s.u.User.Password()
		return p
	}
	return ""
}

func (s *postgresState) dbname() string {
	return strings.TrimPrefix(s.u.Path, "/")
}

package squash

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type postgresState struct {
	u *url.URL
}

func (s *postgresState) Driver() string { return "postgres" }

func (s *postgresState) Dump(path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	env := s.env()

	// 1. DDL only — no owner/ACL noise, consistent across environments.
	schemaArgs := append(s.baseArgs(),
		"--schema-only",
		"--no-owner",
		"--no-acl",
	)
	out, err := runCmd(env, "pg_dump", schemaArgs...)
	if err != nil {
		return fmt.Errorf("pg_dump schema: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}

	// 2. Append data-only dump of schema_migrations so that a fresh install
	//    restoring this file already knows which migrations are applied.
	dataArgs := append(s.baseArgs(),
		"--data-only",
		"--table="+migrationTable,
	)
	migData, err := runCmd(env, "pg_dump", dataArgs...)
	if err != nil {
		if isMissingTableError(err.Error()) {
			return nil // nothing applied yet — skip silently
		}
		return fmt.Errorf("pg_dump migrations data: %w", err)
	}
	return appendToFile(path, migData)
}

func (s *postgresState) Load(path string) error {
	args := append(s.baseArgs(), "--file="+path, "--no-password")
	cmd := exec.Command("psql", args...)
	cmd.Env = s.env()
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
	}
}

func (s *postgresState) env() []string {
	return append(os.Environ(), "PGPASSWORD="+s.password())
}

func (s *postgresState) host() string {
	if h := s.u.Hostname(); h != "" {
		return h
	}
	return "localhost"
}

func (s *postgresState) port() string {
	if p := s.u.Port(); p != "" {
		return p
	}
	return "5432"
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

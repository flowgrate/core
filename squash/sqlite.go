package squash

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// sqlite_ internal tables to exclude from the dump.
var sqliteInternalRe = regexp.MustCompile(`(?i)CREATE TABLE sqlite_\S+.*?;\s*\n`)

type sqliteState struct {
	dbPath string
}

func (s *sqliteState) Driver() string { return "sqlite" }

func (s *sqliteState) Dump(path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	// 1. Schema DDL — sqlite3 ".schema --indent" gives nicely formatted CREATE
	//    statements. We strip sqlite_* internal tables (same as Laravel).
	out, err := runCmd(nil, "sqlite3", s.dbPath, ".schema --indent")
	if err != nil {
		return fmt.Errorf("sqlite3 schema: %w", err)
	}
	schema := sqliteInternalRe.ReplaceAll(out, nil)
	if err := os.WriteFile(path, append(schema, '\n'), 0o644); err != nil {
		return err
	}

	// 2. Append INSERT rows from schema_migrations.
	//    We use ".dump 'schema_migrations'" and keep only -- comments and
	//    INSERT lines to match Laravel's compact format.
	migOut, err := runCmd(nil, "sqlite3", s.dbPath, ".dump '"+migrationTable+"'")
	if err != nil {
		if isMissingTableError(err.Error()) {
			return nil
		}
		return fmt.Errorf("sqlite3 dump migrations: %w", err)
	}

	var inserts []string
	for _, line := range strings.Split(string(migOut), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Keep only comment lines and INSERT statements (skip PRAGMA, BEGIN, COMMIT, CREATE).
		if strings.HasPrefix(trimmed, "--") || strings.HasPrefix(strings.ToUpper(trimmed), "INSERT") {
			inserts = append(inserts, line)
		}
	}
	if len(inserts) > 0 {
		if err := appendToFile(path, []byte(strings.Join(inserts, "\n")+"\n")); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqliteState) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.Command("sqlite3", s.dbPath)
	cmd.Stdin = f
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

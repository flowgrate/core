package squash

import (
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var autoIncrementRe = regexp.MustCompile(`(?i)\s+AUTO_INCREMENT=\d+`)

type mysqlState struct {
	u          *url.URL
	isMariaDB  bool // forced via scheme (mariadb://)
	_mariaOnce *bool
}

func newMySQLState(u *url.URL, forceMariaDB bool) *mysqlState {
	return &mysqlState{u: u, isMariaDB: forceMariaDB}
}

func (s *mysqlState) Driver() string {
	if s.mariaDB() {
		return "mariadb"
	}
	return "mysql"
}

// mariaDB returns true when the dump client is MariaDB.
// For mysql:// DSN we detect at runtime by inspecting mysqldump --version.
// For mariadb:// DSN it is always true.
func (s *mysqlState) mariaDB() bool {
	if s.isMariaDB {
		return true
	}
	if s._mariaOnce == nil {
		v := false
		out, err := exec.Command("mysqldump", "--version").Output()
		if err == nil {
			v = strings.Contains(strings.ToLower(string(out)), "mariadb")
		}
		s._mariaOnce = &v
	}
	return *s._mariaOnce
}

func (s *mysqlState) Dump(path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	// 1. DDL only.
	out, err := s.runDump(s.schemaDumpArgs())
	if err != nil {
		return err
	}

	// Strip AUTO_INCREMENT counters — they diverge between environments and
	// cause noise in version control (same behaviour as Laravel).
	out = autoIncrementRe.ReplaceAll(out, nil)

	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}

	// 2. Append migration table rows (no DDL, compact one-INSERT-per-row format).
	migOut, err := s.runDump(s.migrationDumpArgs())
	if err != nil {
		if isMissingTableError(err.Error()) {
			return nil
		}
		return err
	}
	return appendToFile(path, migOut)
}

func (s *mysqlState) Load(path string) error {
	client := "mysql"
	if s.mariaDB() {
		client = "mariadb"
	}
	args := append(s.connArgs(), "--database="+s.dbname())
	cmd := exec.Command(client, args...)
	cmd.Env = s.env()
	cmd.Stdin, _ = os.Open(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// schemaDumpArgs returns flags for the DDL-only dump.
// We mimic Laravel's flag set and implement the same retry logic for
// --column-statistics (MySQL 8+) and --set-gtid-purged (not on MariaDB).
func (s *mysqlState) schemaDumpArgs() []string {
	args := append(s.connArgs(),
		"--no-tablespaces",
		"--skip-add-locks",
		"--skip-comments",
		"--skip-set-charset",
		"--tz-utc",
		"--no-data",
		"--routines",
	)
	if !s.mariaDB() {
		args = append(args, "--column-statistics=0", "--set-gtid-purged=OFF")
	}
	args = append(args, s.dbname())
	return args
}

// migrationDumpArgs returns flags for the data-only migration table dump.
func (s *mysqlState) migrationDumpArgs() []string {
	args := append(s.connArgs(),
		"--no-create-info",
		"--skip-extended-insert", // one INSERT per row — readable diffs
		"--skip-routines",
		"--compact",
		"--complete-insert",
		s.dbname(),
		migrationTable,
	)
	return args
}

// runDump runs the dump binary (mysqldump / mariadb-dump) with retry logic
// that strips unsupported flags on older clients — same as Laravel's
// executeDumpProcess.
func (s *mysqlState) runDump(args []string) ([]byte, error) {
	bin := "mysqldump"
	if s.mariaDB() {
		bin = "mariadb-dump"
	}
	return s.runWithRetry(bin, args, 0)
}

func (s *mysqlState) runWithRetry(bin string, args []string, depth int) ([]byte, error) {
	if depth > 30 {
		return nil, nil // prevent infinite loop, same guard as Laravel
	}
	out, err := runCmd(s.env(), bin, args...)
	if err == nil {
		return out, nil
	}
	msg := err.Error()
	if strings.Contains(msg, "column-statistics") || strings.Contains(msg, "column_statistics") {
		filtered := filterArg(args, "--column-statistics=0")
		return s.runWithRetry(bin, filtered, depth+1)
	}
	if strings.Contains(msg, "set-gtid-purged") {
		filtered := filterArg(args, "--set-gtid-purged=OFF")
		return s.runWithRetry(bin, filtered, depth+1)
	}
	return nil, err
}

func (s *mysqlState) connArgs() []string {
	args := []string{
		"--host=" + s.host(),
		"--port=" + s.port(),
		"--user=" + s.user(),
	}
	if sock := s.socket(); sock != "" {
		args = append(args, "--socket="+sock)
	}
	return args
}

func (s *mysqlState) env() []string {
	// MYSQL_PWD is the standard env var for avoiding password prompt.
	return append(os.Environ(), "MYSQL_PWD="+s.password())
}

func (s *mysqlState) host() string {
	if h := s.u.Hostname(); h != "" {
		return h
	}
	return "127.0.0.1"
}

func (s *mysqlState) port() string {
	if p := s.u.Port(); p != "" {
		return p
	}
	return "3306"
}

func (s *mysqlState) user() string {
	if s.u.User != nil {
		return s.u.User.Username()
	}
	return ""
}

func (s *mysqlState) password() string {
	if s.u.User != nil {
		p, _ := s.u.User.Password()
		return p
	}
	return ""
}

func (s *mysqlState) dbname() string {
	return strings.TrimPrefix(s.u.Path, "/")
}

// socket returns the Unix socket path if specified as a query param.
func (s *mysqlState) socket() string {
	return s.u.Query().Get("socket")
}

// filterArg returns args without the first occurrence of target.
func filterArg(args []string, target string) []string {
	out := make([]string, 0, len(args))
	removed := false
	for _, a := range args {
		if !removed && a == target {
			removed = true
			continue
		}
		out = append(out, a)
	}
	return out
}

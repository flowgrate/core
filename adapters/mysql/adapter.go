package mysql

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/flowgrate/core/manifest"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) DriverName() string { return "mysql" }

// FormatDSN converts a mysql:// URL to the DSN format expected by go-sql-driver/mysql.
func (a *Adapter) FormatDSN(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port := u.Port()
	if port == "" {
		port = "3306"
	}
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	dbname := strings.TrimPrefix(u.Path, "/")
	// parseTime=true so DATETIME columns scan into time.Time correctly.
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, dbname), nil
}

func (a *Adapter) Compile(op manifest.Operation) ([]string, error) {
	return Compile(op)
}

func (a *Adapter) Placeholder(_ int) string { return "?" }

func (a *Adapter) CreateHistorySQL() string {
	return `CREATE TABLE IF NOT EXISTS schema_migrations (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    migration  VARCHAR(255) NOT NULL UNIQUE,
    batch      INT          NOT NULL,
    applied_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
}

func (a *Adapter) ListTablesSQL() string {
	return `SELECT table_name FROM information_schema.tables
	        WHERE table_schema = DATABASE() AND table_name != 'schema_migrations'`
}

func (a *Adapter) DropTableSQL(table string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", table)
}

// PreDropSQL disables foreign key checks so tables can be dropped in any order.
func (a *Adapter) PreDropSQL() []string {
	return []string{"SET FOREIGN_KEY_CHECKS = 0;"}
}

// PostDropSQL re-enables foreign key checks.
func (a *Adapter) PostDropSQL() []string {
	return []string{"SET FOREIGN_KEY_CHECKS = 1;"}
}

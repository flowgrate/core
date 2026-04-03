package mysql

import (
	"strings"
	"testing"

	"github.com/flowgrate/core/manifest"
)

func intPtr(v int) *int { return &v }

func TestCompileCreateTable(t *testing.T) {
	op := manifest.Operation{
		Action: "create_table",
		Table:  "users",
		Columns: []manifest.Column{
			{Name: "id", Type: "big_integer", Primary: true, AutoIncrement: true},
			{Name: "name", Type: "string", Length: intPtr(255)},
			{Name: "active", Type: "boolean", Default: true},
			{Name: "role_id", Type: "big_integer"},
			{Name: "created_at", Type: "timestamp"},
		},
		Indexes: []manifest.Index{
			{Columns: []string{"name"}, Unique: true, Name: "uq_users_name"},
			{Columns: []string{"role_id"}, Unique: false},
		},
		ForeignKeys: []manifest.ForeignKey{
			{Column: "role_id", ReferencesTable: "roles", ReferencesColumn: "id", OnDelete: "cascade"},
		},
	}

	sqls, err := Compile(op)
	if err != nil {
		t.Fatal(err)
	}

	// CREATE TABLE + 1 non-unique index
	if len(sqls) != 2 {
		t.Fatalf("expected 2 statements, got %d:\n%v", len(sqls), sqls)
	}

	create := sqls[0]
	assertContains(t, create, "CREATE TABLE users")
	assertContains(t, create, "ENGINE=InnoDB")
	// MySQL uses BIGINT AUTO_INCREMENT, not BIGSERIAL
	assertContains(t, create, "id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY")
	assertContains(t, create, "name VARCHAR(255) NOT NULL")
	// MySQL boolean = TINYINT(1), true default = 1
	assertContains(t, create, "active TINYINT(1) NOT NULL DEFAULT 1")
	assertContains(t, create, "CONSTRAINT fk_users_role_id FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE")
	// MySQL uses UNIQUE KEY, not CONSTRAINT ... UNIQUE
	assertContains(t, create, "UNIQUE KEY uq_users_name (name)")

	assertContains(t, sqls[1], "CREATE INDEX idx_users_role_id ON users (role_id);")
}

func TestCompileCreateTable_AllTypes(t *testing.T) {
	op := manifest.Operation{
		Action: "create_table",
		Table:  "products",
		Columns: []manifest.Column{
			{Name: "a", Type: "small_integer"},
			{Name: "b", Type: "integer"},
			{Name: "c", Type: "big_integer"},
			{Name: "d", Type: "decimal", Precision: intPtr(10), Scale: intPtr(2)},
			{Name: "e", Type: "float"},
			{Name: "f", Type: "double"},
			{Name: "g", Type: "text"},
			{Name: "h", Type: "uuid"},
			{Name: "i", Type: "json"},
			{Name: "j", Type: "jsonb"},
			{Name: "k", Type: "binary"},
			{Name: "l", Type: "date"},
			{Name: "m", Type: "time"},
			{Name: "n", Type: "timestamp"},
		},
	}

	sqls, err := Compile(op)
	if err != nil {
		t.Fatal(err)
	}
	create := sqls[0]

	assertContains(t, create, "a SMALLINT")
	assertContains(t, create, "b INT")
	assertContains(t, create, "c BIGINT")
	assertContains(t, create, "d DECIMAL(10, 2)")
	assertContains(t, create, "e FLOAT")
	assertContains(t, create, "f DOUBLE")
	assertContains(t, create, "g LONGTEXT")
	// UUID → CHAR(36)
	assertContains(t, create, "h CHAR(36)")
	assertContains(t, create, "i JSON")
	// JSONB falls back to JSON in MySQL
	assertContains(t, create, "j JSON")
	assertContains(t, create, "k LONGBLOB")
	assertContains(t, create, "l DATE")
	assertContains(t, create, "m TIME")
	// timestamp → DATETIME in MySQL
	assertContains(t, create, "n DATETIME")
}

func TestCompileAlterTable(t *testing.T) {
	op := manifest.Operation{
		Action: "alter_table",
		Table:  "users",
		Columns: []manifest.Column{
			{Name: "status", Type: "string", Length: intPtr(50), ColumnAction: "add", Default: "active"},
			{Name: "name", Type: "string", Length: intPtr(500), ColumnAction: "change", Nullable: true},
			{Name: "email", ColumnAction: "drop"},
		},
	}

	sqls, err := Compile(op)
	if err != nil {
		t.Fatal(err)
	}

	// add: 1, change: 1 (MODIFY COLUMN, not two statements like postgres), drop: 1
	if len(sqls) != 3 {
		t.Fatalf("expected 3 statements, got %d:\n%v", len(sqls), sqls)
	}

	assertContains(t, sqls[0], "ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'active'")
	// MySQL uses MODIFY COLUMN for type changes
	assertContains(t, sqls[1], "MODIFY COLUMN name VARCHAR(500) NULL")
	assertContains(t, sqls[2], "DROP COLUMN email")
}

func TestCompileDropTable(t *testing.T) {
	sqls, _ := Compile(manifest.Operation{Action: "drop_table", Table: "users"})
	assertContains(t, sqls[0], "DROP TABLE users;")

	sqls, _ = Compile(manifest.Operation{Action: "drop_table", Table: "users", IfExists: true})
	assertContains(t, sqls[0], "DROP TABLE IF EXISTS users;")
}

func assertContains(t *testing.T, sql, substr string) {
	t.Helper()
	if !strings.Contains(sql, substr) {
		t.Errorf("expected SQL to contain:\n  %q\ngot:\n  %q", substr, sql)
	}
}

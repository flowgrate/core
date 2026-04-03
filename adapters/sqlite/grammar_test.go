package sqlite

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
	// SQLite auto-increment primary key
	assertContains(t, create, "id INTEGER PRIMARY KEY AUTOINCREMENT")
	assertContains(t, create, "name TEXT NOT NULL")
	// boolean → INTEGER, true default → 1
	assertContains(t, create, "active INTEGER NOT NULL DEFAULT 1")
	assertContains(t, create, "FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE")
	assertContains(t, create, "CONSTRAINT uq_users_name UNIQUE (name)")

	assertContains(t, sqls[1], "CREATE INDEX idx_users_role_id ON users (role_id);")
}

func TestCompileCreateTable_AllTypes(t *testing.T) {
	op := manifest.Operation{
		Action: "create_table",
		Table:  "things",
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

	// SQLite type affinity
	assertContains(t, create, "a INTEGER")
	assertContains(t, create, "b INTEGER")
	assertContains(t, create, "c INTEGER")
	assertContains(t, create, "d REAL")
	assertContains(t, create, "e REAL")
	assertContains(t, create, "f REAL")
	assertContains(t, create, "g TEXT")
	assertContains(t, create, "h TEXT") // UUID → TEXT
	assertContains(t, create, "i TEXT") // JSON → TEXT
	assertContains(t, create, "j TEXT") // JSONB → TEXT
	assertContains(t, create, "k BLOB")
	assertContains(t, create, "l TEXT") // date → TEXT (ISO 8601)
	assertContains(t, create, "m TEXT")
	assertContains(t, create, "n TEXT")
}

func TestCompileAlterTable_AddDrop(t *testing.T) {
	op := manifest.Operation{
		Action: "alter_table",
		Table:  "users",
		Columns: []manifest.Column{
			{Name: "phone", Type: "string", ColumnAction: "add", Nullable: true},
			{Name: "avatar", ColumnAction: "drop"},
		},
	}

	sqls, err := Compile(op)
	if err != nil {
		t.Fatal(err)
	}

	if len(sqls) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(sqls))
	}
	assertContains(t, sqls[0], "ADD COLUMN phone TEXT NULL")
	assertContains(t, sqls[1], "DROP COLUMN avatar")
}

func TestCompileAlterTable_ChangeUnsupported(t *testing.T) {
	op := manifest.Operation{
		Action: "alter_table",
		Table:  "users",
		Columns: []manifest.Column{
			{Name: "name", Type: "string", ColumnAction: "change"},
		},
	}

	_, err := Compile(op)
	if err == nil {
		t.Fatal("expected error for ALTER COLUMN in SQLite, got nil")
	}
	if !strings.Contains(err.Error(), "SQLite does not support ALTER COLUMN") {
		t.Errorf("unexpected error message: %v", err)
	}
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

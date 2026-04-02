package postgres

import (
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
			{Name: "active", Type: "boolean", Default: false},
			{Name: "role_id", Type: "big_integer"},
			{Name: "created_at", Type: "timestamp", Default: "NOW()"},
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
	assertContains(t, create, "id BIGSERIAL PRIMARY KEY")
	assertContains(t, create, "name VARCHAR(255) NOT NULL")
	assertContains(t, create, "active BOOLEAN NOT NULL DEFAULT false")
	assertContains(t, create, "CONSTRAINT fk_users_role_id FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE")
	assertContains(t, create, "CONSTRAINT uq_users_name UNIQUE (name)")

	index := sqls[1]
	assertContains(t, index, "CREATE INDEX idx_users_role_id ON users (role_id);")
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

	// add: 1, change: 2 (TYPE + DROP NOT NULL), drop: 1 = 4
	if len(sqls) != 4 {
		t.Fatalf("expected 4 statements, got %d:\n%v", len(sqls), sqls)
	}

	assertContains(t, sqls[0], "ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'active'")
	assertContains(t, sqls[1], "ALTER COLUMN name TYPE VARCHAR(500)")
	assertContains(t, sqls[2], "ALTER COLUMN name DROP NOT NULL")
	assertContains(t, sqls[3], "DROP COLUMN email")
}

func TestCompileDropTable(t *testing.T) {
	sqls, _ := Compile(manifest.Operation{Action: "drop_table", Table: "users"})
	assertContains(t, sqls[0], "DROP TABLE users;")

	sqls, _ = Compile(manifest.Operation{Action: "drop_table", Table: "users", IfExists: true})
	assertContains(t, sqls[0], "DROP TABLE IF EXISTS users;")
}

func assertContains(t *testing.T, sql, substr string) {
	t.Helper()
	if !contains(sql, substr) {
		t.Errorf("expected SQL to contain:\n  %q\ngot:\n  %q", substr, sql)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

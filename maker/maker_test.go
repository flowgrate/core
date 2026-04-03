package maker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		kind    migrationKind
		table   string
		column  string
	}{
		{"CreateUsersTable", kindCreate, "users", ""},
		{"CreateUserProfilesTable", kindCreate, "user_profiles", ""},
		{"DropPostsTable", kindDropTable, "posts", ""},
		{"AddStatusToUsers", kindAddColumn, "users", "status"},
		{"AddProfileImageToUserProfiles", kindAddColumn, "user_profiles", "profile_image"},
		{"ChangeEmailInUsers", kindChangeColumn, "users", "email"},
		{"DropAvatarFromUsers", kindDropColumn, "users", "avatar"},
		{"SomethingRandom", kindBlank, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := parse(tc.name)
			if m.Kind != tc.kind {
				t.Errorf("kind: want %d, got %d", tc.kind, m.Kind)
			}
			if m.Table != tc.table {
				t.Errorf("table: want %q, got %q", tc.table, m.Table)
			}
			if m.Column != tc.column {
				t.Errorf("column: want %q, got %q", tc.column, m.Column)
			}
		})
	}
}

func TestToSnake(t *testing.T) {
	cases := [][2]string{
		{"Users", "users"},
		{"UserProfiles", "user_profiles"},
		{"NmStatistics", "nm_statistics"},
		{"Status", "status"},
	}
	for _, tc := range cases {
		if got := toSnake(tc[0]); got != tc[1] {
			t.Errorf("toSnake(%q) = %q, want %q", tc[0], got, tc[1])
		}
	}
}

func TestMake_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	path, err := Make("CreateUsersTable", dir, "csharp", MakeOptions{})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	assertContains(t, content, "class CreateUsersTable : Migration")
	assertContains(t, content, `Schema.Create("users"`)
	assertContains(t, content, `Schema.DropIfExists("users")`)
	assertContains(t, content, "table.Id()")
	assertContains(t, content, "table.Timestamps()")
}

func TestMake_AddColumn(t *testing.T) {
	dir := t.TempDir()
	path, err := Make("AddStatusToUsers", dir, "csharp", MakeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	content := string(mustRead(t, path))
	assertContains(t, content, `Schema.Table("users"`)
	assertContains(t, content, `table.AddColumn("status")`)
	assertContains(t, content, `table.DropColumn("status")`)
}

func TestMake_TimestampInFilename(t *testing.T) {
	dir := t.TempDir()
	path, err := Make("CreateUsersTable", dir, "csharp", MakeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	base := path[strings.LastIndex(path, "/")+1:]
	// filename: 20240101_120000_CreateUsersTable.cs
	if !strings.HasSuffix(base, "_CreateUsersTable.cs") {
		t.Errorf("unexpected filename: %s", base)
	}
	if len(base) < len("20060102_150405_CreateUsersTable.cs") {
		t.Errorf("filename too short: %s", base)
	}
}

func TestMake_PythonTemplate(t *testing.T) {
	dir := t.TempDir()
	path, err := Make("CreateUsersTable", dir, "python", MakeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	content := string(mustRead(t, path))
	assertContains(t, content, "class CreateUsersTable(Migration):")
	assertContains(t, content, `Schema.create("users")`)
	assertContains(t, content, `Schema.drop_if_exists("users")`)
	assertContains(t, content, "table.id()")
	assertContains(t, content, "table.timestamps()")
	if !strings.HasSuffix(path, ".py") {
		t.Errorf("expected .py extension, got: %s", path)
	}
}

func TestMake_CustomSDK_StubA(t *testing.T) {
	dir := t.TempDir()
	path, err := Make("CreateUsersTable", dir, "ruby", MakeOptions{})
	if err != nil {
		t.Fatalf("unexpected error for custom sdk: %v", err)
	}
	// Variant A: generates a .migration JSON skeleton
	if !strings.HasSuffix(path, ".migration") {
		t.Errorf("expected .migration extension, got %s", path)
	}
	content, _ := os.ReadFile(path)
	assertContains(t, string(content), `"action": "create_table"`)
}

func TestMake_CustomSDK_StubB(t *testing.T) {
	stubsDir := t.TempDir()
	// Write a custom create.tmpl
	os.WriteFile(filepath.Join(stubsDir, "create.tmpl"),
		[]byte("# {{.ClassName}} — create {{.Table}}\n"), 0o644)

	dir := t.TempDir()
	path, err := Make("CreateUsersTable", dir, "ruby", MakeOptions{
		StubsDir: stubsDir,
		FileExt:  ".rb",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, ".rb") {
		t.Errorf("expected .rb extension, got %s", path)
	}
	content, _ := os.ReadFile(path)
	assertContains(t, string(content), "CreateUsersTable")
	assertContains(t, string(content), "users")
}

func assertContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected to contain %q\ngot:\n%s", sub, s)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

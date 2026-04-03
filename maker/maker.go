package maker

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"
)

// MakeOptions carries optional settings for custom SDK support.
type MakeOptions struct {
	StubsDir  string // Variant B: directory containing .tmpl files
	FileExt   string // Variant B: override file extension
	TableCase string // "snake" (default) | "camel"
}

type migrationKind int

const (
	kindCreate migrationKind = iota
	kindAddColumn
	kindChangeColumn
	kindDropColumn
	kindDropTable
	kindBlank
)

type meta struct {
	Kind      migrationKind
	ClassName string
	Namespace string
	Version   string
	Table     string
	Column    string
}

type langTemplates struct {
	Create        string
	DropTable     string
	AddColumn     string
	ChangeColumn  string
	DropColumn    string
	Blank         string
	FileExt       string
	NamespaceFunc func(datePrefix string) string
}

func Make(name, projectDir, sdk string, opts MakeOptions) (string, error) {
	now := time.Now()
	timestamp := now.Format("20060102_150405")
	datePrefix := now.Format("20060102")

	lang, err := resolveLang(sdk, opts)
	if err != nil {
		return "", err
	}

	nameFunc := resolveNameFunc(opts.TableCase)

	m := parse(name)
	m.ClassName = name
	m.Version = timestamp
	m.Namespace = lang.NamespaceFunc(datePrefix)
	m.Table = nameFunc(m.Table)
	m.Column = nameFunc(m.Column)

	tmpl, err := chooseTemplate(m, lang)
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%s_%s%s", timestamp, name, lang.FileExt)
	path := filepath.Join(projectDir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, m); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	return path, nil
}

func resolveLang(sdk string, opts MakeOptions) (langTemplates, error) {
	switch sdk {
	case "csharp", "":
		return csharpTemplates, nil
	case "python":
		return pythonTemplates, nil
	default:
		// Variant B: load templates from a user-supplied stubs directory.
		if opts.StubsDir != "" {
			return loadStubsFromDir(opts.StubsDir, opts.FileExt)
		}
		// Variant A: generate a JSON manifest skeleton as a .migration file.
		lang := stubTemplates
		if opts.FileExt != "" {
			lang.FileExt = opts.FileExt
		}
		return lang, nil
	}
}

// loadStubsFromDir loads langTemplates from a directory of named .tmpl files.
// Expected files: create.tmpl, drop_table.tmpl, add_column.tmpl,
// change_column.tmpl, drop_column.tmpl, blank.tmpl
// All files are optional; missing ones fall back to stubTemplates.
func loadStubsFromDir(dir, fileExt string) (langTemplates, error) {
	lang := stubTemplates // start from stub defaults
	if fileExt != "" {
		lang.FileExt = fileExt
	}

	names := map[string]*string{
		"create.tmpl":        &lang.Create,
		"drop_table.tmpl":    &lang.DropTable,
		"add_column.tmpl":    &lang.AddColumn,
		"change_column.tmpl": &lang.ChangeColumn,
		"drop_column.tmpl":   &lang.DropColumn,
		"blank.tmpl":         &lang.Blank,
	}

	for filename, dst := range names {
		path := filepath.Join(dir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // missing template → use stub default
			}
			return langTemplates{}, fmt.Errorf("read stub %s: %w", path, err)
		}
		*dst = string(data)
	}

	return lang, nil
}

func parse(name string) meta {
	// Create{X}Table
	if strings.HasPrefix(name, "Create") && strings.HasSuffix(name, "Table") {
		table := name[len("Create") : len(name)-len("Table")]
		return meta{Kind: kindCreate, Table: table}
	}

	// Drop{X}Table
	if strings.HasPrefix(name, "Drop") && strings.HasSuffix(name, "Table") {
		table := name[len("Drop") : len(name)-len("Table")]
		return meta{Kind: kindDropTable, Table: table}
	}

	// Add{Column}To{Table}
	if strings.HasPrefix(name, "Add") {
		if col, table, ok := splitOn(name[len("Add"):], "To"); ok {
			return meta{Kind: kindAddColumn, Column: col, Table: table}
		}
	}

	// Change{Column}In{Table}
	if strings.HasPrefix(name, "Change") {
		if col, table, ok := splitOn(name[len("Change"):], "In"); ok {
			return meta{Kind: kindChangeColumn, Column: col, Table: table}
		}
	}

	// Drop{Column}From{Table}
	if strings.HasPrefix(name, "Drop") {
		if col, table, ok := splitOn(name[len("Drop"):], "From"); ok {
			return meta{Kind: kindDropColumn, Column: col, Table: table}
		}
	}

	return meta{Kind: kindBlank}
}

// splitOn splits "StatusToUsers" on the first occurrence of sep ("To") into ("Status", "Users").
func splitOn(s, sep string) (left, right string, ok bool) {
	re := regexp.MustCompile(sep + `[A-Z]`)
	loc := re.FindStringIndex(s)
	if loc == nil {
		return "", "", false
	}
	return s[:loc[0]], s[loc[0]+len(sep):], true
}

// toSnake converts PascalCase to snake_case: "UserProfiles" → "user_profiles"
func toSnake(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			result = append(result, '_')
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

// toCamel converts PascalCase to camelCase: "UserProfiles" → "userProfiles"
func toCamel(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// resolveNameFunc returns the naming function for the given table_case setting.
func resolveNameFunc(tableCase string) func(string) string {
	if tableCase == "camel" {
		return toCamel
	}
	return toSnake // default
}

func chooseTemplate(m meta, lang langTemplates) (*template.Template, error) {
	var src string
	switch m.Kind {
	case kindCreate:
		src = lang.Create
	case kindDropTable:
		src = lang.DropTable
	case kindAddColumn:
		src = lang.AddColumn
	case kindChangeColumn:
		src = lang.ChangeColumn
	case kindDropColumn:
		src = lang.DropColumn
	default:
		src = lang.Blank
	}
	return template.New("migration").Parse(src)
}

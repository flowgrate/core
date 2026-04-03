package sqlite

import (
	"fmt"
	"strings"

	"github.com/flowgrate/core/manifest"
)

// Compile converts a single manifest operation into one or more SQL statements.
func Compile(op manifest.Operation) ([]string, error) {
	switch op.Action {
	case "create_table":
		return compileCreate(op)
	case "alter_table":
		return compileAlter(op)
	case "drop_table":
		return compileDrop(op)
	default:
		return nil, fmt.Errorf("unknown action: %s", op.Action)
	}
}

// --- CREATE TABLE ---

func compileCreate(op manifest.Operation) ([]string, error) {
	var parts []string

	for _, col := range op.Columns {
		def, err := columnDef(col)
		if err != nil {
			return nil, err
		}
		parts = append(parts, "    "+def)
	}

	for _, fk := range op.ForeignKeys {
		parts = append(parts, "    "+foreignKeyDef(op.Table, fk))
	}

	for _, idx := range op.Indexes {
		if idx.Unique {
			parts = append(parts, "    "+uniqueDef(op.Table, idx))
		}
	}

	sql := fmt.Sprintf("CREATE TABLE %s (\n%s\n);", op.Table, strings.Join(parts, ",\n"))
	result := []string{sql}

	for _, idx := range op.Indexes {
		if !idx.Unique {
			result = append(result, indexStatement(op.Table, idx))
		}
	}

	return result, nil
}

func columnDef(col manifest.Column) (string, error) {
	if col.ColumnAction == "drop" {
		return "", fmt.Errorf("drop column not valid inside CREATE TABLE")
	}

	sqlType, err := mapType(col)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s", col.Name, sqlType)

	if col.Primary {
		// INTEGER PRIMARY KEY is SQLite's canonical auto-increment rowid alias.
		sb.WriteString(" PRIMARY KEY AUTOINCREMENT")
	} else if !col.Nullable {
		sb.WriteString(" NOT NULL")
	} else {
		sb.WriteString(" NULL")
	}

	if col.DefaultExpression != "" {
		fmt.Fprintf(&sb, " DEFAULT (%s)", col.DefaultExpression)
	} else if col.Default != nil {
		fmt.Fprintf(&sb, " DEFAULT %s", formatDefault(col.Default))
	}

	return sb.String(), nil
}

func foreignKeyDef(table string, fk manifest.ForeignKey) string {
	def := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)",
		fk.Column, fk.ReferencesTable, fk.ReferencesColumn)
	if fk.OnUpdate != "" {
		def += " ON UPDATE " + strings.ToUpper(fk.OnUpdate)
	}
	if fk.OnDelete != "" {
		def += " ON DELETE " + strings.ToUpper(fk.OnDelete)
	}
	return def
}

func uniqueDef(table string, idx manifest.Index) string {
	name := idx.Name
	if name == "" {
		name = fmt.Sprintf("uq_%s_%s", table, strings.Join(idx.Columns, "_"))
	}
	return fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)", name, strings.Join(idx.Columns, ", "))
}

func indexStatement(table string, idx manifest.Index) string {
	name := idx.Name
	if name == "" {
		name = fmt.Sprintf("idx_%s_%s", table, strings.Join(idx.Columns, "_"))
	}
	return fmt.Sprintf("CREATE INDEX %s ON %s (%s);", name, table, strings.Join(idx.Columns, ", "))
}

// --- ALTER TABLE ---

func compileAlter(op manifest.Operation) ([]string, error) {
	var statements []string

	for _, col := range op.Columns {
		switch col.ColumnAction {
		case "add", "":
			def, err := columnDef(col)
			if err != nil {
				return nil, err
			}
			statements = append(statements, fmt.Sprintf(
				"ALTER TABLE %s ADD COLUMN %s;", op.Table, def))

		case "change":
			// SQLite does not support ALTER COLUMN natively.
			// A full table-rebuild is required; Flowgrate does not do this automatically.
			return nil, fmt.Errorf(
				"SQLite does not support ALTER COLUMN for %q — rebuild the table manually", col.Name)

		case "drop":
			// Supported since SQLite 3.35.0 (2021-03-12).
			statements = append(statements, fmt.Sprintf(
				"ALTER TABLE %s DROP COLUMN %s;", op.Table, col.Name))
		}
	}

	return statements, nil
}

// --- DROP TABLE ---

func compileDrop(op manifest.Operation) ([]string, error) {
	if op.IfExists {
		return []string{fmt.Sprintf("DROP TABLE IF EXISTS %s;", op.Table)}, nil
	}
	return []string{fmt.Sprintf("DROP TABLE %s;", op.Table)}, nil
}

// --- Type mapping ---

// SQLite uses type affinity: TEXT, INTEGER, REAL, NUMERIC, BLOB.
func mapType(col manifest.Column) (string, error) {
	switch col.Type {
	case "string", "text", "uuid", "json", "jsonb":
		return "TEXT", nil
	case "small_integer", "integer", "boolean":
		return "INTEGER", nil
	case "big_integer":
		return "INTEGER", nil
	case "decimal", "float", "double":
		return "REAL", nil
	case "date", "time", "timestamp":
		// SQLite stores dates as TEXT in ISO 8601 format.
		return "TEXT", nil
	case "binary":
		return "BLOB", nil
	default:
		return "", fmt.Errorf("unknown column type: %s", col.Type)
	}
}

func formatDefault(v any) string {
	switch val := v.(type) {
	case bool:
		if val {
			return "1"
		}
		return "0"
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%g", val)
	case string:
		return fmt.Sprintf("'%s'", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

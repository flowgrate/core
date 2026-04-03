package mysql

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

	sql := fmt.Sprintf("CREATE TABLE %s (\n%s\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;",
		op.Table, strings.Join(parts, ",\n"))
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
		sb.WriteString(" NOT NULL AUTO_INCREMENT PRIMARY KEY")
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
	name := fmt.Sprintf("fk_%s_%s", table, fk.Column)
	def := fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
		name, fk.Column, fk.ReferencesTable, fk.ReferencesColumn)
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
	return fmt.Sprintf("UNIQUE KEY %s (%s)", name, strings.Join(idx.Columns, ", "))
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
			// MySQL uses MODIFY COLUMN to change type/constraints in-place.
			sqlType, err := mapType(col)
			if err != nil {
				return nil, err
			}
			nullability := "NOT NULL"
			if col.Nullable {
				nullability = "NULL"
			}
			statements = append(statements, fmt.Sprintf(
				"ALTER TABLE %s MODIFY COLUMN %s %s %s;",
				op.Table, col.Name, sqlType, nullability))

		case "drop":
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

func mapType(col manifest.Column) (string, error) {
	switch col.Type {
	case "string":
		length := 255
		if col.Length != nil {
			length = *col.Length
		}
		return fmt.Sprintf("VARCHAR(%d)", length), nil
	case "text":
		return "LONGTEXT", nil
	case "small_integer":
		return "SMALLINT", nil
	case "integer":
		return "INT", nil
	case "big_integer":
		return "BIGINT", nil
	case "decimal":
		precision := 8
		scale := 2
		if col.Precision != nil {
			precision = *col.Precision
		}
		if col.Scale != nil {
			scale = *col.Scale
		}
		return fmt.Sprintf("DECIMAL(%d, %d)", precision, scale), nil
	case "float":
		return "FLOAT", nil
	case "double":
		return "DOUBLE", nil
	case "boolean":
		// TINYINT(1) is the standard boolean representation in MySQL.
		return "TINYINT(1)", nil
	case "date":
		return "DATE", nil
	case "time":
		return "TIME", nil
	case "timestamp":
		return "DATETIME", nil
	case "uuid":
		// MySQL has no native UUID type; CHAR(36) stores the canonical text form.
		return "CHAR(36)", nil
	case "json":
		return "JSON", nil
	case "jsonb":
		// MySQL has no JSONB; fall back to JSON.
		return "JSON", nil
	case "binary":
		return "LONGBLOB", nil
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

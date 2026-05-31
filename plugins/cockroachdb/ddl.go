package cockroachdb

import (
	"fmt"
	"strings"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
)

const (
	constraintPrimaryKey = "primary_key"
	constraintUnique     = "unique"
	constraintCheck      = "check"
	constraintForeignKey = "foreign_key"
)

var validOnDelete = map[string]bool{
	"NO ACTION":   true,
	"RESTRICT":    true,
	"CASCADE":     true,
	"SET NULL":    true,
	"SET DEFAULT": true,
}

// renameTableSQL builds ALTER TABLE ... RENAME TO ... with both identifiers
// validated and quoted (the new name is bare — RENAME TO cannot move the table
// to another schema).
func renameTableSQL(schema, table, newName string) (string, error) {
	to, err := sqldb.SafeIdentifier(newName)
	if err != nil {
		return "", err
	}
	return "ALTER TABLE " + sqldb.Qualified(schema, table) + " RENAME TO " + sqldb.QuoteIdent(to), nil
}

func renameColumnSQL(schema, table, column, newName string) (string, error) {
	col, err := sqldb.SafeIdentifier(column)
	if err != nil {
		return "", err
	}
	to, err := sqldb.SafeIdentifier(newName)
	if err != nil {
		return "", err
	}
	return "ALTER TABLE " + sqldb.Qualified(schema, table) + " RENAME COLUMN " + sqldb.QuoteIdent(col) + " TO " + sqldb.QuoteIdent(to), nil
}

func alterColumnTypeSQL(schema, table, column, newType, using string) (string, error) {
	col, err := sqldb.SafeIdentifier(column)
	if err != nil {
		return "", err
	}
	dataType := strings.TrimSpace(newType)
	if !sqldb.SafeType(dataType) {
		return "", fmt.Errorf("%w: unsafe column type", plugin.ErrInvalidInput)
	}
	stmt := "ALTER TABLE " + sqldb.Qualified(schema, table) + " ALTER COLUMN " + sqldb.QuoteIdent(col) + " TYPE " + dataType
	if u := strings.TrimSpace(using); u != "" {
		if !sqldb.SafeDefault(u) {
			return "", fmt.Errorf("%w: unsafe USING expression", plugin.ErrInvalidInput)
		}
		stmt += " USING " + u
	}
	return stmt, nil
}

func dropConstraintSQL(schema, table, name string) (string, error) {
	con, err := sqldb.SafeIdentifier(name)
	if err != nil {
		return "", err
	}
	return "ALTER TABLE " + sqldb.Qualified(schema, table) + " DROP CONSTRAINT " + sqldb.QuoteIdent(con), nil
}

type constraintRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Columns    any    `json:"columns"`
	Check      string `json:"check"`
	RefTable   string `json:"refTable"`
	RefColumns string `json:"refColumns"`
	OnDelete   string `json:"onDelete"`
}

// addConstraintSQL builds ALTER TABLE ... ADD CONSTRAINT for PK/UNIQUE/CHECK/FK.
// Identifiers are validated and quoted; the CHECK expression is value-free and
// passes the same conservative safety gate as a column default.
func addConstraintSQL(schema, table string, req constraintRequest) (string, error) {
	name, err := sqldb.SafeIdentifier(req.Name)
	if err != nil {
		return "", err
	}
	prefix := "ALTER TABLE " + sqldb.Qualified(schema, table) + " ADD CONSTRAINT " + sqldb.QuoteIdent(name) + " "
	switch req.Type {
	case constraintPrimaryKey, constraintUnique:
		cols, err := sqldb.IdentifierListValue(req.Columns, sqldb.QuoteIdent)
		if err != nil {
			return "", err
		}
		keyword := "PRIMARY KEY"
		if req.Type == constraintUnique {
			keyword = "UNIQUE"
		}
		return prefix + keyword + " (" + strings.Join(cols, ", ") + ")", nil
	case constraintCheck:
		expr := strings.TrimSpace(req.Check)
		if expr == "" {
			return "", fmt.Errorf("%w: check expression is required", plugin.ErrInvalidInput)
		}
		if !sqldb.SafeDefault(expr) {
			return "", fmt.Errorf("%w: unsafe check expression", plugin.ErrInvalidInput)
		}
		return prefix + "CHECK (" + expr + ")", nil
	case constraintForeignKey:
		cols, err := sqldb.IdentifierListValue(req.Columns, sqldb.QuoteIdent)
		if err != nil {
			return "", err
		}
		refTable, err := qualifiedRef(req.RefTable)
		if err != nil {
			return "", err
		}
		refCols, err := sqldb.IdentifierList(req.RefColumns, sqldb.QuoteIdent)
		if err != nil {
			return "", err
		}
		stmt := prefix + "FOREIGN KEY (" + strings.Join(cols, ", ") + ") REFERENCES " + refTable + " (" + strings.Join(refCols, ", ") + ")"
		if onDelete := strings.ToUpper(strings.TrimSpace(req.OnDelete)); onDelete != "" {
			if !validOnDelete[onDelete] {
				return "", fmt.Errorf("%w: unsupported ON DELETE action", plugin.ErrInvalidInput)
			}
			stmt += " ON DELETE " + onDelete
		}
		return stmt, nil
	default:
		return "", fmt.Errorf("%w: unsupported constraint type", plugin.ErrInvalidInput)
	}
}

// qualifiedRef parses a foreign-key target table given either bare ("orders")
// or schema-qualified ("public.orders"), validating and quoting each part.
func qualifiedRef(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: referenced table is required", plugin.ErrInvalidInput)
	}
	if schema, table, ok := strings.Cut(raw, "."); ok {
		s, err := sqldb.SafeIdentifier(schema)
		if err != nil {
			return "", err
		}
		t, err := sqldb.SafeIdentifier(table)
		if err != nil {
			return "", err
		}
		return sqldb.Qualified(s, t), nil
	}
	t, err := sqldb.SafeIdentifier(raw)
	if err != nil {
		return "", err
	}
	return sqldb.QuoteIdent(t), nil
}

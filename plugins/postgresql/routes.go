package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
)

type row map[string]any

type actionResult struct {
	OK bool `json:"ok"`
}

type confirmationError struct {
	message string
}

func (e confirmationError) Error() string { return e.message }

var dialect = sqldb.Dialect{QuoteIdent: sqldb.QuoteIdent, Placeholder: sqldb.DollarPlaceholder}

func routes() []plugin.Route {
	return []plugin.Route{
		// Sidebar tree: Databases -> Schemas -> Tables/Views.
		{ID: "postgresql.tree.databases", Method: plugin.MethodGet, Path: "/tree/databases", Permission: "postgresql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.tree.databases", Handle: treeDatabases},
		{ID: "postgresql.tree.schemas", Method: plugin.MethodGet, Path: "/tree/schemas", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.tree.schemas", Handle: treeSchemas},
		{ID: "postgresql.tree.relations", Method: plugin.MethodGet, Path: "/tree/relations", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.tree.relations", Handle: treeRelations},

		// Catalog lists.
		{ID: "postgresql.databases.list", Method: plugin.MethodGet, Path: "/databases", Permission: "postgresql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.databases.list", Handle: listDatabases},
		{ID: "postgresql.database.overview", Method: plugin.MethodGet, Path: "/databases/{database}/overview", Permission: "postgresql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.database.overview", Handle: databaseOverview},
		{ID: "postgresql.schemas.list", Method: plugin.MethodGet, Path: "/schemas", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.schemas.list", Handle: listSchemas},
		{ID: "postgresql.schema.overview", Method: plugin.MethodGet, Path: "/schemas/{schema}/overview", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.schema.overview", Handle: schemaOverview},
		{ID: "postgresql.tables.list", Method: plugin.MethodGet, Path: "/tables", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.tables.list", Handle: listTables},
		{ID: "postgresql.relations.graph", Method: plugin.MethodGet, Path: "/relations/graph", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.relations.graph", Handle: relationGraph},
		{ID: "postgresql.views.list", Method: plugin.MethodGet, Path: "/views", Permission: "postgresql.views.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.views.list", Handle: listViews},
		{ID: "postgresql.view.drop", Method: plugin.MethodDelete, Path: "/views/{schema}/{view}", Permission: "postgresql.views.delete", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.view.drop", Handle: dropView},
		{ID: "postgresql.functions.list", Method: plugin.MethodGet, Path: "/functions", Permission: "postgresql.functions.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.functions.list", Handle: listFunctions},
		{ID: "postgresql.sequences.list", Method: plugin.MethodGet, Path: "/sequences", Permission: "postgresql.sequences.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.sequences.list", Handle: listSequences},

		// Table data + structure.
		{ID: "postgresql.table.rows", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/rows", Permission: "postgresql.tables.data.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.rows", Handle: tableRows},
		{ID: "postgresql.view.rows", Method: plugin.MethodGet, Path: "/views/{schema}/{table}/rows", Permission: "postgresql.views.data.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.view.rows", Handle: tableRows},
		{ID: "postgresql.table.columns", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/columns", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.columns", Handle: tableColumnsRoute},
		{ID: "postgresql.table.indexes", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/indexes", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.indexes", Handle: tableIndexes},
		{ID: "postgresql.table.constraints", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/constraints", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.constraints", Handle: tableConstraints},
		{ID: "postgresql.table.ddl", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/ddl", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.ddl", Handle: tableDDL},
		{ID: "postgresql.view.definition", Method: plugin.MethodGet, Path: "/views/{schema}/{table}/definition", Permission: "postgresql.views.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.view.definition", Handle: viewDefinition},
		{ID: "postgresql.function.definition", Method: plugin.MethodGet, Path: "/functions/{oid}/definition", Permission: "postgresql.functions.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.function.definition", Handle: functionDefinition},
		{ID: "postgresql.sequence.overview", Method: plugin.MethodGet, Path: "/sequences/{schema}/{table}/overview", Permission: "postgresql.sequences.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.sequence.overview", Handle: sequenceOverview},
		{ID: "postgresql.completion", Method: plugin.MethodGet, Path: "/completion", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.completion", Handle: completionRoute},

		// Inline data-grid row mutations.
		{ID: "postgresql.table.row.insert", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/rows", Permission: "postgresql.tables.data.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.table.row.insert", Handle: insertRow},
		{ID: "postgresql.table.row.update", Method: plugin.MethodPatch, Path: "/tables/{schema}/{table}/rows", Permission: "postgresql.tables.data.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.table.row.update", Handle: updateRow},
		{ID: "postgresql.table.row.delete", Method: plugin.MethodDelete, Path: "/tables/{schema}/{table}/rows", Permission: "postgresql.tables.data.delete", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.table.row.delete", Handle: deleteRow},

		// DDL.
		{ID: "postgresql.database.create", Method: plugin.MethodPost, Path: "/databases", Permission: "postgresql.databases.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.database.create", Input: databaseCreateSchema(), Handle: createDatabase},
		{ID: "postgresql.database.drop", Method: plugin.MethodDelete, Path: "/databases/{database}", Permission: "postgresql.databases.delete", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.database.drop", Handle: dropDatabase},
		{ID: "postgresql.schema.create", Method: plugin.MethodPost, Path: "/schemas", Permission: "postgresql.schemas.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.schema.create", Input: schemaCreateSchema(), Handle: createSchema},
		{ID: "postgresql.schema.drop", Method: plugin.MethodDelete, Path: "/schemas/{schema}", Permission: "postgresql.schemas.delete", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.schema.drop", Handle: dropSchema},
		{ID: "postgresql.table.create", Method: plugin.MethodPost, Path: "/schemas/{schema}/tables", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.table.create", Input: tableCreateSchema(), Handle: createTable},
		{ID: "postgresql.column.add", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/columns", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.column.add", Input: columnAddSchema(), Handle: addColumn},
		{ID: "postgresql.column.drop", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/columns/drop", Permission: "postgresql.tables.write", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.column.drop", Handle: dropColumn},
		{ID: "postgresql.column.rename", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/columns/rename", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.column.rename", Input: columnRenameSchema(), Handle: renameColumn},
		{ID: "postgresql.column.alter", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/columns/alter", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.column.alter", Input: columnAlterSchema(), Handle: alterColumn},
		{ID: "postgresql.constraint.add", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/constraints", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.constraint.add", Input: constraintAddSchema(), Handle: addConstraint},
		{ID: "postgresql.constraint.drop", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/constraints/drop", Permission: "postgresql.tables.write", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.constraint.drop", Handle: dropConstraint},
		{ID: "postgresql.table.rename", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/rename", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.table.rename", Input: tableRenameSchema(), Handle: renameTable},
		{ID: "postgresql.index.create", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/indexes", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.index.create", Input: indexCreateSchema(), Handle: createIndex},
		{ID: "postgresql.index.drop", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/indexes/drop", Permission: "postgresql.tables.write", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.index.drop", Handle: dropIndex},
		{ID: "postgresql.table.truncate", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/truncate", Permission: "postgresql.tables.delete", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.table.truncate", Handle: truncateTable},
		{ID: "postgresql.table.drop", Method: plugin.MethodDelete, Path: "/tables/{schema}/{table}", Permission: "postgresql.tables.delete", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.table.drop", Handle: dropTable},

		// Query editor (scoped to the active database).
		{ID: "postgresql.query", Method: plugin.MethodWS, Path: "/query", Permission: "postgresql.query.execute", Risk: plugin.RiskPrivileged, AuditEvent: "postgresql.query", Stream: queryStream},
		{ID: "postgresql.query.cancel", Method: plugin.MethodPost, Path: "/query/cancel", Permission: "postgresql.query.cancel", Risk: plugin.RiskWrite, AuditEvent: "postgresql.query.cancel", Handle: cancelQuery},
	}
}

func pgSession(rc *plugin.RequestContext) (*Session, error) {
	return unwrap(rc.Session)
}

// paramOf reads a value that may arrive as a path param ({name}) or as a plain
// query param (p.name) when the route template does not declare it.
func paramOf(rc *plugin.RequestContext, name string) string {
	if v := rc.Param(name); v != "" {
		return v
	}
	return rc.Query().Get("p." + name)
}

// dbPool resolves the session plus the pool for the request's target database
// (the "database" param; empty means the connection's configured database).
func dbPool(rc *plugin.RequestContext) (*Session, *pgxpool.Pool, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, nil, err
	}
	pool, err := s.poolFor(rc.Ctx, paramOf(rc, "database"))
	if err != nil {
		return nil, nil, err
	}
	return s, pool, nil
}

func databaseCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Database", Fields: []plugin.Field{
		{Key: "name", Label: "Database name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "owner", Label: "Owner", Type: plugin.FieldText, Help: "Optional role that owns the new database."},
	}}}}
}

func schemaCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Schema", Fields: []plugin.Field{
		{Key: "name", Label: "Schema name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
	}}}}
}

func tableCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Table", Fields: []plugin.Field{
		{Key: "name", Label: "Table name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "columns", Label: "Columns", Type: plugin.FieldJSON, Required: true, Help: `Array of {"name":"id","type":"bigserial","primary":true,"nullable":false}`},
		{Key: "if_not_exists", Label: "If not exists", Type: plugin.FieldToggle, Default: true},
	}}}}
}

func columnAddSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Column", Fields: []plugin.Field{
		{Key: "name", Label: "Column name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "type", Label: "Type", Type: plugin.FieldText, Required: true, Default: "text"},
		{Key: "nullable", Label: "Nullable", Type: plugin.FieldToggle, Default: true},
		{Key: "default", Label: "Default expression", Type: plugin.FieldText},
	}}}}
}

func columnRenameSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Rename column", Fields: []plugin.Field{
		{Key: "newName", Label: "New name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
	}}}}
}

func columnAlterSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Change type", Fields: []plugin.Field{
		{Key: "type", Label: "New type", Type: plugin.FieldText, Required: true, Default: "text"},
		{Key: "using", Label: "USING expression", Type: plugin.FieldText, Help: "Optional cast expression, e.g. column::integer."},
	}}}}
}

func tableRenameSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Rename table", Fields: []plugin.Field{
		{Key: "newName", Label: "New name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
	}}}}
}

func constraintAddSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Constraint", Fields: []plugin.Field{
		{Key: "name", Label: "Constraint name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "type", Label: "Type", Type: plugin.FieldSelect, Required: true, Default: constraintPrimaryKey, Options: []plugin.Option{
			{Label: "Primary key", Value: constraintPrimaryKey},
			{Label: "Unique", Value: constraintUnique},
			{Label: "Check", Value: constraintCheck},
			{Label: "Foreign key", Value: constraintForeignKey},
		}},
		{Key: "columns", Label: "Columns", Type: plugin.FieldMultiSelect, OptionsSource: &plugin.DataSource{RouteID: "postgresql.table.columns", Params: tableParams()}, Help: "Columns for primary key, unique, or the referencing side of a foreign key.", VisibleWhen: &plugin.Condition{AnyOf: []plugin.Rule{
			{Field: "type", Op: plugin.OpIn, Value: []any{constraintPrimaryKey, constraintUnique, constraintForeignKey}},
		}}},
		{Key: "check", Label: "Check expression", Type: plugin.FieldText, Help: "Boolean expression, e.g. price > 0.", VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "type", Op: plugin.OpEq, Value: constraintCheck}}}},
		{Key: "refTable", Label: "Referenced table", Type: plugin.FieldText, Help: "Target table for a foreign key (schema-qualified or bare).", VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "type", Op: plugin.OpEq, Value: constraintForeignKey}}}},
		{Key: "refColumns", Label: "Referenced columns", Type: plugin.FieldText, Help: "Comma-separated columns on the referenced table.", VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "type", Op: plugin.OpEq, Value: constraintForeignKey}}}},
		{Key: "onDelete", Label: "On delete", Type: plugin.FieldSelect, Options: []plugin.Option{
			{Label: "No action", Value: "NO ACTION"},
			{Label: "Restrict", Value: "RESTRICT"},
			{Label: "Cascade", Value: "CASCADE"},
			{Label: "Set null", Value: "SET NULL"},
			{Label: "Set default", Value: "SET DEFAULT"},
		}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "type", Op: plugin.OpEq, Value: constraintForeignKey}}}},
	}}}}
}

func indexCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Index", Fields: []plugin.Field{
		{Key: "name", Label: "Index name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "columns", Label: "Columns", Type: plugin.FieldMultiSelect, Required: true, OptionsSource: &plugin.DataSource{RouteID: "postgresql.table.columns", Params: tableParams()}},
		{Key: "unique", Label: "Unique", Type: plugin.FieldToggle},
	}}}}
}

// --- tree ---------------------------------------------------------------

func treeDatabases(rc *plugin.RequestContext) (any, error) {
	res, err := listDatabases(rc)
	if err != nil {
		return nil, err
	}
	page := res.(plugin.Page[row])
	nodes := make([]plugin.TreeNode, 0, len(page.Items))
	for _, r := range page.Items {
		name := fmt.Sprint(r["name"])
		nodes = append(nodes, plugin.TreeNode{
			Key:            "db:" + name,
			Label:          name,
			Icon:           icon("database"),
			Ref:            &plugin.ResourceRef{Kind: "database", Name: name, UID: name},
			ChildrenSource: &plugin.DataSource{RouteID: "postgresql.tree.schemas", Params: map[string]string{"database": name}},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func treeSchemas(rc *plugin.RequestContext) (any, error) {
	database := paramOf(rc, "database")
	res, err := listSchemas(rc)
	if err != nil {
		return nil, err
	}
	page := res.(plugin.Page[row])
	nodes := make([]plugin.TreeNode, 0, len(page.Items))
	for _, r := range page.Items {
		name := fmt.Sprint(r["name"])
		nodes = append(nodes, plugin.TreeNode{
			Key:            "db:" + database + ":schema:" + name,
			Label:          name,
			Icon:           icon("folder-tree"),
			Ref:            &plugin.ResourceRef{Kind: "schema", Scope: database, Name: name, UID: database + "." + name},
			ChildrenSource: &plugin.DataSource{RouteID: "postgresql.tree.relations", Params: map[string]string{"database": database, "schema": name}},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func treeRelations(rc *plugin.RequestContext) (any, error) {
	database := paramOf(rc, "database")
	schema := paramOf(rc, "schema")
	_, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT c.relname AS name, c.relkind::text AS kind
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1 AND c.relkind IN ('r','p','v','m')
ORDER BY c.relname`, []any{schema})
	if err != nil {
		return nil, err
	}
	nodes := make([]plugin.TreeNode, 0, len(rows))
	for _, r := range rows {
		name := fmt.Sprint(r["name"])
		kind, iconName := "table", "table-2"
		if k := fmt.Sprint(r["kind"]); k == "v" || k == "m" {
			kind, iconName = "view", "panel-top"
		}
		nodes = append(nodes, plugin.TreeNode{
			Key:   "db:" + database + ":rel:" + schema + "." + name,
			Label: name,
			Icon:  icon(iconName),
			Ref:   &plugin.ResourceRef{Kind: kind, Scope: database, Namespace: schema, Name: name, UID: database + "." + schema + "." + name},
			Leaf:  true,
		})
	}
	total := len(nodes)
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: &total}, nil
}

// --- catalog lists ------------------------------------------------------

func listDatabases(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT d.datname AS name, pg_get_userbyid(d.datdba) AS owner, pg_database_size(d.datname) AS size,
       pg_encoding_to_char(d.encoding) AS encoding
FROM pg_database d
WHERE d.datallowconn AND NOT d.datistemplate
ORDER BY d.datname`, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name := fmt.Sprint(r["name"])
		r["ref"] = plugin.ResourceRef{Kind: "database", Name: name, UID: name}
	}
	return pageRows(rc, rows)
}

func databaseOverview(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT current_database() AS name, current_user AS "user", version() AS version,
       pg_database_size(current_database()) AS size,
       (SELECT COUNT(*) FROM pg_namespace WHERE nspname !~ '^pg_' AND nspname <> 'information_schema') AS schemas,
       (SELECT COUNT(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE c.relkind IN ('r','p') AND n.nspname !~ '^pg_') AS tables`, nil)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

func listSchemas(rc *plugin.RequestContext) (any, error) {
	database := paramOf(rc, "database")
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT n.nspname AS name, pg_get_userbyid(n.nspowner) AS owner,
       COUNT(c.oid) FILTER (WHERE c.relkind IN ('r','p')) AS tables,
       COUNT(c.oid) FILTER (WHERE c.relkind = 'v') AS views,
       (SELECT COUNT(*) FROM pg_proc p WHERE p.pronamespace = n.oid) AS functions
FROM pg_namespace n
LEFT JOIN pg_class c ON c.relnamespace = n.oid
WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
GROUP BY n.oid, n.nspname, n.nspowner
ORDER BY n.nspname`, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name := fmt.Sprint(r["name"])
		r["ref"] = plugin.ResourceRef{Kind: "schema", Scope: database, Name: name, UID: database + "." + name}
	}
	return pageRows(rc, rows)
}

func schemaOverview(rc *plugin.RequestContext) (any, error) {
	schema, err := sqldb.SafeIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return nil, err
	}
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT n.nspname AS name, pg_get_userbyid(n.nspowner) AS owner,
       COUNT(c.oid) FILTER (WHERE c.relkind IN ('r','p')) AS tables,
       COUNT(c.oid) FILTER (WHERE c.relkind = 'v') AS views,
       COUNT(c.oid) FILTER (WHERE c.relkind = 'S') AS sequences,
       (SELECT COUNT(*) FROM pg_proc p WHERE p.pronamespace = n.oid) AS functions
FROM pg_namespace n
LEFT JOIN pg_class c ON c.relnamespace = n.oid
WHERE n.nspname = $1
GROUP BY n.oid, n.nspname, n.nspowner`, []any{schema})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

func listTables(rc *plugin.RequestContext) (any, error) {
	return relationList(rc, []string{"r", "p"}, "table")
}

const relationGraphSQL = `
SELECT con.conname AS constraint_name,
       cn.nspname AS child_schema, cc.relname AS child_table, ca.attname AS child_column,
       pn.nspname AS parent_schema, pc.relname AS parent_table, pa.attname AS parent_column
FROM pg_constraint con
JOIN pg_class cc ON cc.oid = con.conrelid
JOIN pg_namespace cn ON cn.oid = cc.relnamespace
JOIN pg_class pc ON pc.oid = con.confrelid
JOIN pg_namespace pn ON pn.oid = pc.relnamespace
JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS ck(attnum, ord) ON true
JOIN LATERAL unnest(con.confkey) WITH ORDINALITY AS pk(attnum, ord) ON pk.ord = ck.ord
JOIN pg_attribute ca ON ca.attrelid = con.conrelid AND ca.attnum = ck.attnum
JOIN pg_attribute pa ON pa.attrelid = con.confrelid AND pa.attnum = pk.attnum
WHERE con.contype = 'f' AND cn.nspname !~ '^pg_' AND cn.nspname <> 'information_schema'
  AND ($1::text = '' OR cn.nspname = $1)
ORDER BY con.conname, ck.ord`

const relationColumnsSQL = `
SELECT table_schema, table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema !~ '^pg_' AND table_schema <> 'information_schema'
  AND ($1::text = '' OR table_schema = $1)
ORDER BY table_schema, table_name, ordinal_position`

func relationGraph(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	schema, err := sqldb.OptionalIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return nil, err
	}
	colRows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, relationColumnsSQL, []any{schema})
	if err != nil {
		return nil, err
	}
	fkRows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, relationGraphSQL, []any{schema})
	if err != nil {
		return nil, err
	}
	columns := make([]sqldb.TableColumn, 0, len(colRows))
	for _, r := range colRows {
		columns = append(columns, sqldb.TableColumnFromRow(r))
	}
	fks := make([]sqldb.ForeignKey, 0, len(fkRows))
	for _, r := range fkRows {
		fks = append(fks, sqldb.ForeignKeyFromRow(r))
	}
	return sqldb.RelationGraph(columns, fks), nil
}

func listViews(rc *plugin.RequestContext) (any, error) {
	return relationList(rc, []string{"v", "m"}, "view")
}

func relationList(rc *plugin.RequestContext, kinds []string, refKind string) (any, error) {
	database := paramOf(rc, "database")
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	schema, err := sqldb.OptionalIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT c.relname AS name, n.nspname AS schema, pg_get_userbyid(c.relowner) AS owner,
       c.reltuples::bigint AS rows, pg_total_relation_size(c.oid) AS size,
       CASE WHEN c.relkind IN ('v','m') THEN false ELSE NULL END AS updatable
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = ANY($1) AND n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
  AND ($2::text = '' OR n.nspname = $2)
ORDER BY n.nspname, c.relname`, []any{kinds, schema})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name, ns := fmt.Sprint(r["name"]), fmt.Sprint(r["schema"])
		r["ref"] = plugin.ResourceRef{Kind: refKind, Scope: database, Namespace: ns, Name: name, UID: database + "." + ns + "." + name}
	}
	return pageRows(rc, rows)
}

func listFunctions(rc *plugin.RequestContext) (any, error) {
	database := paramOf(rc, "database")
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	schema, err := sqldb.OptionalIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT p.oid::text AS oid, p.proname AS name, n.nspname AS schema,
       pg_get_function_arguments(p.oid) AS arguments,
       pg_get_function_result(p.oid) AS returns,
       l.lanname AS language
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
JOIN pg_language l ON l.oid = p.prolang
WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
  AND ($1::text = '' OR n.nspname = $1)
ORDER BY n.nspname, p.proname`, []any{schema})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name, ns, oid := fmt.Sprint(r["name"]), fmt.Sprint(r["schema"]), fmt.Sprint(r["oid"])
		r["ref"] = plugin.ResourceRef{Kind: "function", Scope: database, Namespace: ns, Name: name, UID: oid}
	}
	return pageRows(rc, rows)
}

func listSequences(rc *plugin.RequestContext) (any, error) {
	database := paramOf(rc, "database")
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	schema, err := sqldb.OptionalIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT sequence_name AS name, sequence_schema AS schema, data_type AS "dataType",
       start_value AS start, increment AS increment
FROM information_schema.sequences
WHERE ($1::text = '' OR sequence_schema = $1)
ORDER BY sequence_schema, sequence_name`, []any{schema})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name, ns := fmt.Sprint(r["name"]), fmt.Sprint(r["schema"])
		r["ref"] = plugin.ResourceRef{Kind: "sequence", Scope: database, Namespace: ns, Name: name, UID: database + "." + ns + "." + name}
	}
	return pageRows(rc, rows)
}

// --- table data ---------------------------------------------------------

func tableRows(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 || limit > s.opts.RowLimit {
		limit = s.opts.RowLimit
	}
	offset, err := cursorOffset(req.Cursor)
	if err != nil {
		return nil, err
	}
	qualified := sqldb.Qualified(schema, table)
	// Free-text search (the grid's filter box): match the whole row's text form,
	// so any column containing the term matches without naming columns.
	filter := req.Search()
	where := ""
	if filter != "" {
		where = " WHERE t::text ILIKE "
	}
	var total int
	countArgs := []any{}
	countSQL := "SELECT COUNT(*) FROM " + qualified + " AS t"
	if filter != "" {
		countSQL += where + "$1"
		countArgs = append(countArgs, "%"+filter+"%")
	}
	if err := pool.QueryRow(rc.Ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, pgErr(err)
	}
	orderBy := ""
	if len(req.Sort) > 0 {
		col, err := sqldb.SafeIdentifier(req.Sort[0].Field)
		if err != nil {
			return nil, err
		}
		dir := "ASC"
		if req.Sort[0].Desc {
			dir = "DESC"
		}
		orderBy = " ORDER BY " + sqldb.QuoteIdent(col) + " " + dir
	}
	dataArgs := []any{limit, offset}
	dataWhere := ""
	if filter != "" {
		dataWhere = where + "$3"
		dataArgs = append(dataArgs, "%"+filter+"%")
	}
	sqlText := "SELECT * FROM " + qualified + " AS t" + dataWhere + orderBy + " LIMIT $1 OFFSET $2"
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, sqlText, dataArgs)
	if err != nil {
		return nil, err
	}
	pk, err := primaryKeyColumns(rc.Ctx, pool, s.opts.QueryTimeout, schema, table)
	if err != nil {
		return nil, err
	}
	attachRowKeys(rows, pk, s.opts.RedactPatterns)
	fks, err := foreignKeys(rc.Ctx, pool, s.opts.QueryTimeout, paramOf(rc, "database"), schema, table)
	if err != nil {
		return nil, err
	}
	attachForeignKeys(rows, fks)
	redactRows(rows, s.opts.RedactPatterns)
	next := ""
	if offset+len(rows) < total {
		next = strconv.Itoa(offset + len(rows))
	}
	return plugin.Page[row]{Items: rows, NextCursor: next, Total: &total}, nil
}

// foreignKeys maps each FK column of a table to the ResourceRef of the table it
// references, so the grid can turn those cells into navigation links.
func foreignKeys(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration, database, schema, table string) (map[string]plugin.ResourceRef, error) {
	rows, err := queryRows(ctx, pool, timeout, `
SELECT kcu.column_name AS col, ccu.table_schema AS ref_schema, ccu.table_name AS ref_table
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_name = tc.constraint_name AND kcu.constraint_schema = tc.constraint_schema
JOIN information_schema.constraint_column_usage ccu
  ON ccu.constraint_name = tc.constraint_name AND ccu.constraint_schema = tc.constraint_schema
WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = $1 AND tc.table_name = $2`, []any{schema, table})
	if err != nil {
		return nil, err
	}
	out := map[string]plugin.ResourceRef{}
	for _, r := range rows {
		col, refSchema, refTable := fmt.Sprint(r["col"]), fmt.Sprint(r["ref_schema"]), fmt.Sprint(r["ref_table"])
		out[col] = plugin.ResourceRef{Kind: "table", Scope: database, Namespace: refSchema, Name: refTable, UID: database + "." + refSchema + "." + refTable}
	}
	return out, nil
}

// attachForeignKeys tags rows with the table-level link map (column -> referenced
// table ref) under the generic "_links" field the grid renders as links.
func attachForeignKeys(rows []row, fks map[string]plugin.ResourceRef) {
	if len(fks) == 0 {
		return
	}
	for _, r := range rows {
		r["_links"] = fks
	}
}

// attachRowKeys tags each row with the primary-key/value map the editable grid
// echoes back for UPDATE/DELETE. The grid stays read-only when the table has no
// primary key, or when a key column is itself sensitive (so a redacted value is
// never shipped raw inside _key).
func attachRowKeys(rows []row, pk, patterns []string) {
	if len(pk) == 0 || sqldb.AnyColumnRedacted(pk, patterns) {
		return
	}
	for _, r := range rows {
		key := map[string]any{}
		for _, col := range pk {
			key[col] = r[col]
		}
		r["_key"] = key
	}
}

func primaryKeyColumns(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration, schema, table string) ([]string, error) {
	rows, err := queryRows(ctx, pool, timeout, `
SELECT a.attname AS name
FROM pg_index i
JOIN pg_class c ON c.oid = i.indrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
WHERE i.indisprimary AND n.nspname = $1 AND c.relname = $2
ORDER BY array_position(i.indkey, a.attnum)`, []any{schema, table})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprint(r["name"]))
	}
	return out, nil
}

func tableColumnsRoute(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT column_name AS name, data_type AS type, is_nullable = 'YES' AS nullable,
       column_default AS default, identity_generation AS identity, ordinal_position AS position
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`, []any{schema, table})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		name := fmt.Sprint(rows[i]["name"])
		rows[i]["ref"] = plugin.ResourceRef{Kind: "column", Scope: schema, Namespace: table, Name: name, UID: schema + "." + table + "." + name}
	}
	return pageRows(rc, rows)
}

func tableIndexes(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT i.relname AS name, ix.indisunique AS unique, ix.indisprimary AS primary,
       pg_get_indexdef(ix.indexrelid) AS definition
FROM pg_index ix
JOIN pg_class t ON t.oid = ix.indrelid
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE n.nspname = $1 AND t.relname = $2
ORDER BY i.relname`, []any{schema, table})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		name := fmt.Sprint(rows[i]["name"])
		rows[i]["ref"] = plugin.ResourceRef{Kind: "index", Scope: schema, Namespace: table, Name: name, UID: schema + "." + table + "." + name}
	}
	return pageRows(rc, rows)
}

func tableConstraints(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT con.conname AS name,
       CASE con.contype WHEN 'p' THEN 'primary key' WHEN 'f' THEN 'foreign key' WHEN 'u' THEN 'unique' WHEN 'c' THEN 'check' ELSE con.contype::text END AS type,
       pg_get_constraintdef(con.oid) AS definition
FROM pg_constraint con
JOIN pg_class t ON t.oid = con.conrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE n.nspname = $1 AND t.relname = $2
ORDER BY con.conname`, []any{schema, table})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		name := fmt.Sprint(rows[i]["name"])
		rows[i]["ref"] = plugin.ResourceRef{Kind: "constraint", Scope: schema, Namespace: table, Name: name, UID: schema + "." + table + "." + name}
	}
	return pageRows(rc, rows)
}

func tableDDL(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	cols, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT column_name AS name, data_type AS type, is_nullable = 'YES' AS nullable, column_default AS default
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`, []any{schema, table})
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, plugin.ErrNotFound
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n", sqldb.Qualified(schema, table))
	for i, c := range cols {
		line := "    " + sqldb.QuoteIdent(fmt.Sprint(c["name"])) + " " + fmt.Sprint(c["type"])
		if c["nullable"] == false {
			line += " NOT NULL"
		}
		if d, ok := c["default"].(string); ok && d != "" {
			line += " DEFAULT " + d
		}
		if i < len(cols)-1 {
			line += ","
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(");")
	return row{"schema": schema, "name": table, "definition": b.String()}, nil
}

func viewDefinition(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	_, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	var def sql.NullString
	if err := pool.QueryRow(rc.Ctx, `SELECT pg_get_viewdef(($1 || '.' || $2)::regclass, true)`, schema, table).Scan(&def); err != nil {
		return nil, pgErr(err)
	}
	return row{"schema": schema, "name": table, "definition": def.String}, nil
}

func functionDefinition(rc *plugin.RequestContext) (any, error) {
	oid, err := strconv.Atoi(rc.Param("oid"))
	if err != nil || oid <= 0 {
		return nil, fmt.Errorf("%w: function oid is invalid", plugin.ErrInvalidInput)
	}
	_, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	var def string
	if err := pool.QueryRow(rc.Ctx, `SELECT pg_get_functiondef($1::oid)`, oid).Scan(&def); err != nil {
		return nil, pgErr(err)
	}
	return row{"definition": def}, nil
}

func sequenceOverview(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT * FROM information_schema.sequences
WHERE sequence_schema = $1 AND sequence_name = $2`, []any{schema, table})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

// --- row mutations ------------------------------------------------------

func insertRow(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var m sqldb.RowMutation
	if err := rc.Bind(&m); err != nil {
		return nil, err
	}
	stmt, args, err := dialect.Insert(sqldb.Qualified(schema, table), m.Values)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, stmt, args...); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func updateRow(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var m sqldb.RowMutation
	if err := rc.Bind(&m); err != nil {
		return nil, err
	}
	if err := validateRowKey(rc, pool, s, schema, table, m.Key); err != nil {
		return nil, err
	}
	stmt, args, err := dialect.Update(sqldb.Qualified(schema, table), m.Key, m.Values)
	if err != nil {
		return nil, err
	}
	tag, err := pool.Exec(rc.Ctx, stmt, args...)
	if err != nil {
		return nil, pgErr(err)
	}
	return singleRowResult(tag.RowsAffected())
}

func deleteRow(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var m sqldb.RowMutation
	if err := rc.Bind(&m); err != nil {
		return nil, err
	}
	if err := validateRowKey(rc, pool, s, schema, table, m.Key); err != nil {
		return nil, err
	}
	stmt, args, err := dialect.Delete(sqldb.Qualified(schema, table), m.Key)
	if err != nil {
		return nil, err
	}
	tag, err := pool.Exec(rc.Ctx, stmt, args...)
	if err != nil {
		return nil, pgErr(err)
	}
	return singleRowResult(tag.RowsAffected())
}

// validateRowKey loads the table's primary key and rejects any client key that
// is not exactly that key, so a mutation can only ever target one row.
func validateRowKey(rc *plugin.RequestContext, pool *pgxpool.Pool, s *Session, schema, table string, key map[string]any) error {
	pk, err := primaryKeyColumns(rc.Ctx, pool, s.opts.QueryTimeout, schema, table)
	if err != nil {
		return err
	}
	return sqldb.ValidateRowKey(pk, key)
}

func singleRowResult(affected int64) (any, error) {
	if affected == 0 {
		return nil, fmt.Errorf("%w: row no longer matches (it may have changed)", plugin.ErrNotFound)
	}
	return actionResult{OK: true}, nil
}

// --- DDL ----------------------------------------------------------------

func createDatabase(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	pool, err := s.poolFor(rc.Ctx, "")
	if err != nil {
		return nil, err
	}
	var req struct {
		Name  string `json:"name" validate:"required"`
		Owner string `json:"owner"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := sqldb.SafeIdentifier(req.Name)
	if err != nil {
		return nil, err
	}
	stmt := "CREATE DATABASE " + sqldb.QuoteIdent(name)
	if owner := strings.TrimSpace(req.Owner); owner != "" {
		ownerID, err := sqldb.SafeIdentifier(owner)
		if err != nil {
			return nil, err
		}
		stmt += " OWNER " + sqldb.QuoteIdent(ownerID)
	}
	if _, err := pool.Exec(rc.Ctx, stmt); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropDatabase(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	name, err := sqldb.SafeIdentifier(paramOf(rc, "database"))
	if err != nil {
		return nil, err
	}
	if name == s.baseDB {
		return nil, fmt.Errorf("%w: cannot drop the connected database", plugin.ErrForbidden)
	}
	// Close any pool we opened while browsing the target; an open connection to
	// it makes PostgreSQL reject DROP DATABASE as "in use".
	s.closePool(name)
	pool, err := s.poolFor(rc.Ctx, "")
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, "DROP DATABASE "+sqldb.QuoteIdent(name)); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func createSchema(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	var req struct {
		Name string `json:"name" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := sqldb.SafeIdentifier(req.Name)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, "CREATE SCHEMA IF NOT EXISTS "+sqldb.QuoteIdent(name)); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropSchema(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	name, err := sqldb.SafeIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, "DROP SCHEMA "+sqldb.QuoteIdent(name)); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func createTable(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, err := sqldb.SafeIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return nil, err
	}
	var req struct {
		Name        string `json:"name" validate:"required"`
		Columns     any    `json:"columns" validate:"required"`
		IfNotExists bool   `json:"if_not_exists"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	table, err := sqldb.SafeIdentifier(req.Name)
	if err != nil {
		return nil, err
	}
	columns, err := sqldb.ParseDDLColumns(req.Columns)
	if err != nil {
		return nil, err
	}
	prefix := "CREATE TABLE "
	if req.IfNotExists {
		prefix += "IF NOT EXISTS "
	}
	sqlText := prefix + sqldb.Qualified(schema, table) + " (" + strings.Join(columns, ", ") + ")"
	if _, err := pool.Exec(rc.Ctx, sqlText); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func addColumn(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Name     string `json:"name" validate:"required"`
		Type     string `json:"type" validate:"required"`
		Nullable bool   `json:"nullable"`
		Default  string `json:"default"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	column, err := sqldb.DDLColumn(sqldb.ColumnSpec{Name: req.Name, Type: req.Type, Nullable: req.Nullable, Default: req.Default})
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, "ALTER TABLE "+sqldb.Qualified(schema, table)+" ADD COLUMN "+column); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropColumn(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	column, err := sqldb.SafeIdentifier(rc.Param("name"))
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, "ALTER TABLE "+sqldb.Qualified(schema, table)+" DROP COLUMN "+sqldb.QuoteIdent(column)); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func renameColumn(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		NewName string `json:"newName" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	stmt, err := renameColumnSQL(schema, table, rc.Param("name"), req.NewName)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, stmt); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func alterColumn(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Type  string `json:"type" validate:"required"`
		Using string `json:"using"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	stmt, err := alterColumnTypeSQL(schema, table, rc.Param("name"), req.Type, req.Using)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, stmt); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func renameTable(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		NewName string `json:"newName" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	stmt, err := renameTableSQL(schema, table, req.NewName)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, stmt); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func addConstraint(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var req constraintRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	stmt, err := addConstraintSQL(schema, table, req)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, stmt); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropConstraint(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	stmt, err := dropConstraintSQL(schema, table, rc.Param("name"))
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, stmt); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func createIndex(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Name    string `json:"name" validate:"required"`
		Columns any    `json:"columns" validate:"required"`
		Unique  bool   `json:"unique"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := sqldb.SafeIdentifier(req.Name)
	if err != nil {
		return nil, err
	}
	cols, err := sqldb.IdentifierListValue(req.Columns, sqldb.QuoteIdent)
	if err != nil {
		return nil, err
	}
	unique := ""
	if req.Unique {
		unique = "UNIQUE "
	}
	stmt := "CREATE " + unique + "INDEX " + sqldb.QuoteIdent(name) + " ON " + sqldb.Qualified(schema, table) + " (" + strings.Join(cols, ", ") + ")"
	if _, err := pool.Exec(rc.Ctx, stmt); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropIndex(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, _, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	name, err := sqldb.SafeIdentifier(rc.Param("name"))
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, "DROP INDEX "+sqldb.Qualified(schema, name)); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func truncateTable(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	return execDDL(rc, "TRUNCATE TABLE "+sqldb.Qualified(schema, table))
}

func dropTable(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	return execDDL(rc, "DROP TABLE "+sqldb.Qualified(schema, table))
}

func dropView(rc *plugin.RequestContext) (any, error) {
	schema, err := sqldb.SafeIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return nil, err
	}
	view, err := sqldb.SafeIdentifier(paramOf(rc, "view"))
	if err != nil {
		return nil, err
	}
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	// Regular and materialized views are listed together but dropped with
	// different statements, so resolve the relkind first.
	var relkind string
	if err := pool.QueryRow(rc.Ctx, `SELECT c.relkind::text FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relname = $2`, schema, view).Scan(&relkind); err != nil {
		return nil, pgErr(err)
	}
	stmt := "DROP VIEW " + sqldb.Qualified(schema, view)
	if relkind == "m" {
		stmt = "DROP MATERIALIZED VIEW " + sqldb.Qualified(schema, view)
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, stmt); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func execDDL(rc *plugin.RequestContext, sqlText string) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	if _, err := pool.Exec(rc.Ctx, sqlText); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

// --- query editor -------------------------------------------------------

func cancelQuery(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	return actionResult{OK: s.cancelAll()}, nil
}

func completionRoute(rc *plugin.RequestContext) (any, error) {
	s, pool, err := dbPool(rc)
	if err != nil {
		return nil, err
	}
	items := []sqldb.CompletionItem{
		{Label: "SELECT", Type: "keyword"},
		{Label: "FROM", Type: "keyword"},
		{Label: "WHERE", Type: "keyword"},
		{Label: "ORDER BY", Type: "keyword"},
		{Label: "GROUP BY", Type: "keyword"},
		{Label: "LIMIT", Type: "keyword"},
		{Label: "INSERT", Type: "keyword"},
		{Label: "UPDATE", Type: "keyword"},
		{Label: "DELETE", Type: "keyword"},
		{Label: "CREATE TABLE", Type: "keyword"},
		{Label: "ALTER TABLE", Type: "keyword"},
	}
	rows, err := queryRows(rc.Ctx, pool, s.opts.QueryTimeout, `
SELECT n.nspname AS schema, c.relname AS relation, c.relkind::text AS kind, a.attname AS column
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema' AND c.relkind IN ('r','p','v','m')
ORDER BY n.nspname, c.relname, a.attnum
LIMIT 2000`, nil)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	add := func(item sqldb.CompletionItem) {
		key := item.Type + ":" + item.Label + ":" + item.Detail
		if seen[key] {
			return
		}
		seen[key] = true
		items = append(items, item)
	}
	for _, r := range rows {
		schema := fmt.Sprint(r["schema"])
		relation := fmt.Sprint(r["relation"])
		kind := "table"
		if k := fmt.Sprint(r["kind"]); k == "v" || k == "m" {
			kind = "view"
		}
		add(sqldb.CompletionItem{Label: schema, Type: "namespace", Detail: "schema"})
		add(sqldb.CompletionItem{Label: relation, Type: kind, Detail: schema, Apply: sqldb.QuoteIdent(schema) + "." + sqldb.QuoteIdent(relation)})
		if column := fmt.Sprint(r["column"]); column != "" && column != "<nil>" {
			add(sqldb.CompletionItem{Label: column, Type: "property", Detail: schema + "." + relation})
		}
	}
	return items, nil
}

func queryStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, pool, err := dbPool(rc)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(client)
	enc := json.NewEncoder(client)
	for {
		var req sqldb.QueryRequest
		if err := dec.Decode(&req); err != nil {
			if client.Context().Err() != nil || errors.Is(err, io.EOF) {
				return nil
			}
			if err := enc.Encode(map[string]any{"error": "Invalid query request."}); err != nil {
				return err
			}
			continue
		}
		statements := sqldb.SplitStatements(req.Query)
		result, err := executeQueryRequest(client.Context(), s, pool, req)
		rc.Audit(queryAuditResult(err), sqldb.AuditParams(sqldb.QueryAudit{
			Query:          req.Query,
			Statements:     statements,
			Confirmed:      req.Confirm,
			ReadOnlyMode:   s.opts.ReadOnly,
			RequiresReview: statementsRequireReview(statements),
			RowCount:       result.RowCount,
			ElapsedMS:      result.ElapsedMS,
			CommandTag:     result.CommandTag,
		}), err)
		if err != nil {
			payload := map[string]any{"error": err.Error()}
			var confirmErr confirmationError
			if errors.As(err, &confirmErr) {
				payload["requiresConfirmation"] = true
				payload["confirmMessage"] = "This PostgreSQL statement can change data or schema. Review it before running."
			}
			if err := enc.Encode(payload); err != nil {
				return err
			}
			continue
		}
		if err := enc.Encode(result); err != nil {
			return err
		}
	}
}

func executeQueryRequest(parent context.Context, s *Session, pool *pgxpool.Pool, req sqldb.QueryRequest) (sqldb.QueryResult, error) {
	statements := sqldb.SplitStatements(req.Query)
	if len(statements) == 0 {
		return sqldb.QueryResult{}, fmt.Errorf("%w: query is empty", plugin.ErrInvalidInput)
	}
	if s.opts.ReadOnly {
		for _, st := range statements {
			if !sqldb.IsReadOnlyStatement(st) {
				return sqldb.QueryResult{}, fmt.Errorf("%w: read-only mode blocks write statements", plugin.ErrForbidden)
			}
		}
	}
	if s.opts.RequireConfirm && !req.Confirm {
		for _, st := range statements {
			if sqldb.IsDestructiveStatement(st) {
				return sqldb.QueryResult{}, confirmationError{message: "statement requires confirmation"}
			}
		}
	}
	ctx, cancel := context.WithTimeout(parent, s.opts.QueryTimeout)
	id := req.RequestID
	if id == "" {
		id = uuid.NewString()
	}
	s.track(id, cancel)
	defer func() {
		cancel()
		s.untrack(id)
	}()
	results := make([]sqldb.StatementResult, 0, len(statements))
	for _, st := range statements {
		res, err := executeStatement(ctx, s, pool, st)
		if err != nil {
			return sqldb.QueryResult{}, err
		}
		results = append(results, res)
	}
	out := sqldb.QueryResult{Statements: results}
	if len(results) > 0 {
		last := results[len(results)-1]
		out.Columns = last.Columns
		out.Rows = last.Rows
		out.RowCount = last.RowCount
		out.ElapsedMS = last.ElapsedMS
		out.Statement = last.Statement
		out.CommandTag = last.CommandTag
	}
	return out, nil
}

func executeStatement(ctx context.Context, s *Session, pool *pgxpool.Pool, statement string) (sqldb.StatementResult, error) {
	start := time.Now()
	rows, err := pool.Query(ctx, statement)
	if err != nil {
		return sqldb.StatementResult{}, pgErr(err)
	}
	defer rows.Close()
	out := sqldb.StatementResult{Statement: statement, Columns: fieldNames(rows.FieldDescriptions())}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return sqldb.StatementResult{}, pgErr(err)
		}
		out.Rows = append(out.Rows, sqldb.DisplayValues(out.Columns, values))
		if len(out.Rows) >= s.opts.RowLimit {
			break
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return sqldb.StatementResult{}, pgErr(err)
	}
	tag := rows.CommandTag()
	out.CommandTag = tag.String()
	out.RowCount = tag.RowsAffected()
	if out.RowCount == 0 && len(out.Rows) > 0 {
		out.RowCount = int64(len(out.Rows))
	}
	out.Rows = sqldb.RedactRows(out.Columns, out.Rows, s.opts.RedactPatterns)
	out.ElapsedMS = time.Since(start).Milliseconds()
	return out, nil
}

func queryAuditResult(err error) models.AuditResult {
	if err == nil {
		return models.AuditAllowed
	}
	var confirmErr confirmationError
	if errors.As(err, &confirmErr) {
		return models.AuditDenied
	}
	return models.AuditError
}

func statementsRequireReview(statements []string) bool {
	for _, st := range statements {
		if sqldb.IsDestructiveStatement(st) {
			return true
		}
	}
	return false
}

// --- shared query/paging helpers ----------------------------------------

func queryRows(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration, sqlText string, args []any) ([]row, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rows, err := pool.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, pgErr(err)
	}
	defer rows.Close()
	names := fieldNames(rows.FieldDescriptions())
	out := []row{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, pgErr(err)
		}
		r := row{}
		for i, name := range names {
			if i < len(values) {
				r[name] = sqldb.DisplayValue(name, values[i])
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, pgErr(err)
	}
	return out, nil
}

func redactRows(rows []row, patterns []string) {
	for _, r := range rows {
		for key, value := range r {
			if key == "_key" {
				continue
			}
			if value != nil && sqldb.RedactColumn(key, patterns) {
				r[key] = sqldb.RedactedValue
			}
		}
	}
}

func fieldNames(fields []pgconn.FieldDescription) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.Name)
	}
	return out
}

func pageRows(rc *plugin.RequestContext, rows []row) (plugin.Page[row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[row]{}, err
	}
	rows = filterRows(rows, req.Search())
	sortRows(rows, req.Sort)
	total := len(rows)
	start, err := cursorOffset(req.Cursor)
	if err != nil {
		return plugin.Page[row]{}, err
	}
	if start > len(rows) {
		start = len(rows)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = len(rows) - start
	}
	end := min(start+limit, len(rows))
	next := ""
	if end < len(rows) {
		next = strconv.Itoa(end)
	}
	return plugin.Page[row]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

func filterRows(rows []row, q string) []row {
	return plugin.FilterRows(rows, q)
}

func sortRows(rows []row, keys []plugin.SortKey) {
	if len(keys) == 0 {
		return
	}
	key := keys[0]
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := fmt.Sprint(rows[i][key.Field]), fmt.Sprint(rows[j][key.Field])
		if key.Desc {
			return a > b
		}
		return a < b
	})
}

func cursorOffset(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: cursor must be an offset", plugin.ErrInvalidInput)
	}
	return n, nil
}

func ensureWritable(s *Session) error {
	if s.opts.ReadOnly {
		return fmt.Errorf("%w: read-only mode blocks write operations", plugin.ErrForbidden)
	}
	return nil
}

func pgErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return plugin.ErrNotFound
	}
	return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
}

func tableIdent(rc *plugin.RequestContext) (string, string, error) {
	schema, err := sqldb.SafeIdentifier(paramOf(rc, "schema"))
	if err != nil {
		return "", "", err
	}
	table, err := sqldb.SafeIdentifier(paramOf(rc, "table"))
	if err != nil {
		return "", "", err
	}
	return schema, table, nil
}

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

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

type row map[string]any

type actionResult struct {
	OK bool `json:"ok"`
}

type confirmationError struct {
	message string
}

func (e confirmationError) Error() string { return e.message }

func routes() []plugin.Route {
	return []plugin.Route{
		{ID: "postgresql.databases.tree", Method: plugin.MethodGet, Path: "/tree/databases", Permission: "postgresql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.databases.tree", Handle: treeDatabases},
		{ID: "postgresql.databases.list", Method: plugin.MethodGet, Path: "/databases", Permission: "postgresql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.databases.list", Handle: listDatabases},
		{ID: "postgresql.database.overview", Method: plugin.MethodGet, Path: "/databases/{database}/overview", Permission: "postgresql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.database.overview", Handle: databaseOverview},
		{ID: "postgresql.schemas.tree", Method: plugin.MethodGet, Path: "/tree/schemas", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.schemas.tree", Handle: schemaTree},
		{ID: "postgresql.schema.children", Method: plugin.MethodGet, Path: "/tree/schemas/{schema}", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.schema.children", Handle: schemaChildren},
		{ID: "postgresql.schemas.list", Method: plugin.MethodGet, Path: "/schemas", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.schemas.list", Handle: listSchemas},
		{ID: "postgresql.schema.overview", Method: plugin.MethodGet, Path: "/schemas/{schema}/overview", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.schema.overview", Handle: schemaOverview},
		{ID: "postgresql.tables.tree", Method: plugin.MethodGet, Path: "/tree/tables", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.tables.tree", Handle: treeTables},
		{ID: "postgresql.tables.list", Method: plugin.MethodGet, Path: "/tables", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.tables.list", Handle: listTables},
		{ID: "postgresql.views.tree", Method: plugin.MethodGet, Path: "/tree/views", Permission: "postgresql.views.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.views.tree", Handle: treeViews},
		{ID: "postgresql.views.list", Method: plugin.MethodGet, Path: "/views", Permission: "postgresql.views.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.views.list", Handle: listViews},
		{ID: "postgresql.functions.tree", Method: plugin.MethodGet, Path: "/tree/functions", Permission: "postgresql.functions.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.functions.tree", Handle: treeFunctions},
		{ID: "postgresql.functions.list", Method: plugin.MethodGet, Path: "/functions", Permission: "postgresql.functions.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.functions.list", Handle: listFunctions},
		{ID: "postgresql.sequences.list", Method: plugin.MethodGet, Path: "/sequences", Permission: "postgresql.sequences.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.sequences.list", Handle: listSequences},
		{ID: "postgresql.table.rows", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/rows", Permission: "postgresql.tables.data.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.rows", Handle: tableRows},
		{ID: "postgresql.view.rows", Method: plugin.MethodGet, Path: "/views/{schema}/{table}/rows", Permission: "postgresql.views.data.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.view.rows", Handle: tableRows},
		{ID: "postgresql.table.columns", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/columns", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.columns", Handle: tableColumnsRoute},
		{ID: "postgresql.table.indexes", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/indexes", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.indexes", Handle: tableIndexes},
		{ID: "postgresql.table.constraints", Method: plugin.MethodGet, Path: "/tables/{schema}/{table}/constraints", Permission: "postgresql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.table.constraints", Handle: tableConstraints},
		{ID: "postgresql.view.definition", Method: plugin.MethodGet, Path: "/views/{schema}/{table}/definition", Permission: "postgresql.views.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.view.definition", Handle: viewDefinition},
		{ID: "postgresql.function.definition", Method: plugin.MethodGet, Path: "/functions/{oid}/definition", Permission: "postgresql.functions.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.function.definition", Handle: functionDefinition},
		{ID: "postgresql.sequence.overview", Method: plugin.MethodGet, Path: "/sequences/{schema}/{table}/overview", Permission: "postgresql.sequences.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.sequence.overview", Handle: sequenceOverview},
		{ID: "postgresql.completion", Method: plugin.MethodGet, Path: "/completion", Permission: "postgresql.schemas.read", Risk: plugin.RiskSafe, AuditEvent: "postgresql.completion", Handle: completionRoute},
		{ID: "postgresql.table.create", Method: plugin.MethodPost, Path: "/schemas/{schema}/tables", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.table.create", Input: tableCreateSchema(), Handle: createTable},
		{ID: "postgresql.column.add", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/columns", Permission: "postgresql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "postgresql.column.add", Input: columnAddSchema(), Handle: addColumn},
		{ID: "postgresql.table.truncate", Method: plugin.MethodPost, Path: "/tables/{schema}/{table}/truncate", Permission: "postgresql.tables.delete", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.table.truncate", Handle: truncateTable},
		{ID: "postgresql.table.drop", Method: plugin.MethodDelete, Path: "/tables/{schema}/{table}", Permission: "postgresql.tables.delete", Risk: plugin.RiskDestructive, AuditEvent: "postgresql.table.drop", Handle: dropTable},
		{ID: "postgresql.query", Method: plugin.MethodWS, Path: "/query", Permission: "postgresql.query.execute", Risk: plugin.RiskPrivileged, AuditEvent: "postgresql.query", Stream: queryStream},
		{ID: "postgresql.query.cancel", Method: plugin.MethodPost, Path: "/query/cancel", Permission: "postgresql.query.cancel", Risk: plugin.RiskWrite, AuditEvent: "postgresql.query.cancel", Handle: cancelQuery},
	}
}

func treeDatabases(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "database", "database", "name", listDatabases)
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

func pgSession(rc *plugin.RequestContext) (*Session, error) {
	return unwrap(rc.Session)
}

func listDatabases(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT d.datname AS name, pg_get_userbyid(d.datdba) AS owner, pg_database_size(d.datname) AS size,
       pg_encoding_to_char(d.encoding) AS encoding, COALESCE(ns.schemas, 0) AS schemas
FROM pg_database d
LEFT JOIN (SELECT COUNT(*) AS schemas FROM pg_namespace WHERE nspname !~ '^pg_' AND nspname <> 'information_schema') ns ON true
WHERE d.datallowconn
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

func treeTables(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "table", "table-2", "name", listTables)
}

func treeViews(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "view", "panel-top", "name", listViews)
}

func treeFunctions(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "function", "function-square", "name", listFunctions)
}

func databaseOverview(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
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

func schemaTree(rc *plugin.RequestContext) (any, error) {
	page, err := listSchemas(rc)
	if err != nil {
		return nil, err
	}
	schemas := page.(plugin.Page[row])
	nodes := make([]plugin.TreeNode, 0, len(schemas.Items))
	for _, r := range schemas.Items {
		name := fmt.Sprint(r["name"])
		nodes = append(nodes, plugin.TreeNode{
			Key:            "schema:" + name,
			Label:          name,
			Icon:           icon("folder"),
			Ref:            &plugin.ResourceRef{Kind: "schema", Name: name, UID: name},
			ChildrenSource: &plugin.DataSource{RouteID: "postgresql.schema.children", Params: map[string]string{"schema": name}},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: schemas.NextCursor, Total: schemas.Total}, nil
}

func schemaChildren(rc *plugin.RequestContext) (any, error) {
	schema, err := sqldb.SafeIdentifier(rc.Param("schema"))
	if err != nil {
		return nil, err
	}
	nodes := []plugin.TreeNode{
		{
			Key:   "schema:" + schema + ":tables",
			Label: "Tables",
			Icon:  icon("table-2"),
			Leaf:  true,
			Ref:   &plugin.ResourceRef{Kind: "schema", Name: schema, UID: schema},
		},
		{
			Key:   "schema:" + schema + ":views",
			Label: "Views",
			Icon:  icon("panel-top"),
			Leaf:  true,
			Ref:   &plugin.ResourceRef{Kind: "schema", Name: schema, UID: schema},
		},
		{
			Key:   "schema:" + schema + ":functions",
			Label: "Functions",
			Icon:  icon("function-square"),
			Leaf:  true,
			Ref:   &plugin.ResourceRef{Kind: "schema", Name: schema, UID: schema},
		},
		{
			Key:   "schema:" + schema + ":sequences",
			Label: "Sequences",
			Icon:  icon("list-ordered"),
			Leaf:  true,
			Ref:   &plugin.ResourceRef{Kind: "schema", Name: schema, UID: schema},
		},
	}
	total := len(nodes)
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: &total}, nil
}

func listSchemas(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
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
		r["ref"] = plugin.ResourceRef{Kind: "schema", Name: name, UID: name}
	}
	return pageRows(rc, rows)
}

func schemaOverview(rc *plugin.RequestContext) (any, error) {
	schema, err := sqldb.SafeIdentifier(rc.Param("schema"))
	if err != nil {
		return nil, err
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
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

func listViews(rc *plugin.RequestContext) (any, error) {
	return relationList(rc, []string{"v", "m"}, "view")
}

func relationList(rc *plugin.RequestContext, kinds []string, refKind string) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	schema, err := sqldb.OptionalIdentifier(rc.Query().Get("p.schema"))
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
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
		name, schema := fmt.Sprint(r["name"]), fmt.Sprint(r["schema"])
		r["ref"] = plugin.ResourceRef{Kind: refKind, Namespace: schema, Name: name, UID: schema + "." + name}
	}
	return pageRows(rc, rows)
}

func listFunctions(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	schema, err := sqldb.OptionalIdentifier(rc.Query().Get("p.schema"))
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
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
		name, schema, oid := fmt.Sprint(r["name"]), fmt.Sprint(r["schema"]), fmt.Sprint(r["oid"])
		r["ref"] = plugin.ResourceRef{Kind: "function", Namespace: schema, Name: name, UID: oid}
	}
	return pageRows(rc, rows)
}

func listSequences(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	schema, err := sqldb.OptionalIdentifier(rc.Query().Get("p.schema"))
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT sequence_name AS name, sequence_schema AS schema, data_type AS "dataType",
       start_value AS start, increment AS increment
FROM information_schema.sequences
WHERE ($1::text = '' OR sequence_schema = $1)
ORDER BY sequence_schema, sequence_name`, []any{schema})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name, schema := fmt.Sprint(r["name"]), fmt.Sprint(r["schema"])
		r["ref"] = plugin.ResourceRef{Kind: "sequence", Namespace: schema, Name: name, UID: schema + "." + name}
	}
	return pageRows(rc, rows)
}

func tableRows(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit > s.opts.RowLimit {
		limit = s.opts.RowLimit
	}
	offset, err := cursorOffset(req.Cursor)
	if err != nil {
		return nil, err
	}
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s", sqldb.Qualified(schema, table))
	var total int
	if err := s.pool.QueryRow(rc.Ctx, countSQL).Scan(&total); err != nil {
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
	sqlText := fmt.Sprintf("SELECT * FROM %s%s LIMIT $1 OFFSET $2", sqldb.Qualified(schema, table), orderBy)
	rows, err := queryRows(rc.Ctx, s, sqlText, []any{limit, offset})
	if err != nil {
		return nil, err
	}
	redactRows(rows, s.opts.RedactPatterns)
	next := ""
	if offset+len(rows) < total {
		next = strconv.Itoa(offset + len(rows))
	}
	return plugin.Page[row]{Items: rows, NextCursor: next, Total: &total}, nil
}

func tableColumnsRoute(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT column_name AS name, data_type AS type, is_nullable = 'YES' AS nullable,
       column_default AS default, identity_generation AS identity, ordinal_position AS position
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`, []any{schema, table})
	if err != nil {
		return nil, err
	}
	return pageRows(rc, rows)
}

func tableIndexes(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
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
	return pageRows(rc, rows)
}

func tableConstraints(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
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
	return pageRows(rc, rows)
}

func viewDefinition(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	var def sql.NullString
	if err := s.pool.QueryRow(rc.Ctx, `SELECT pg_get_viewdef(($1 || '.' || $2)::regclass, true)`, schema, table).Scan(&def); err != nil {
		return nil, pgErr(err)
	}
	return row{"schema": schema, "name": table, "definition": def.String}, nil
}

func functionDefinition(rc *plugin.RequestContext) (any, error) {
	oid, err := strconv.Atoi(rc.Param("oid"))
	if err != nil || oid <= 0 {
		return nil, fmt.Errorf("%w: function oid is invalid", plugin.ErrInvalidInput)
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	var def string
	if err := s.pool.QueryRow(rc.Ctx, `SELECT pg_get_functiondef($1::oid)`, oid).Scan(&def); err != nil {
		return nil, pgErr(err)
	}
	return row{"definition": def}, nil
}

func sequenceOverview(rc *plugin.RequestContext) (any, error) {
	schema, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
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

func completionRoute(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
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
	rows, err := queryRows(rc.Ctx, s, `
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
		if fmt.Sprint(r["kind"]) == "v" || fmt.Sprint(r["kind"]) == "m" {
			kind = "view"
		}
		add(sqldb.CompletionItem{Label: schema, Type: "namespace", Detail: "schema"})
		add(sqldb.CompletionItem{Label: relation, Type: kind, Detail: schema, Apply: sqldb.QuoteIdent(schema) + "." + sqldb.QuoteIdent(relation)})
		column := fmt.Sprint(r["column"])
		if column != "" && column != "<nil>" {
			add(sqldb.CompletionItem{Label: column, Type: "property", Detail: schema + "." + relation})
		}
	}
	functions, err := queryRows(rc.Ctx, s, `
SELECT n.nspname AS schema, p.proname AS name
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
ORDER BY n.nspname, p.proname
LIMIT 500`, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range functions {
		add(sqldb.CompletionItem{Label: fmt.Sprint(r["name"]), Type: "function", Detail: fmt.Sprint(r["schema"])})
	}
	return items, nil
}

func createTable(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	schema, err := sqldb.SafeIdentifier(rc.Param("schema"))
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
	if _, err := s.pool.Exec(rc.Ctx, sqlText); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func addColumn(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
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
	if _, err := s.pool.Exec(rc.Ctx, "ALTER TABLE "+sqldb.Qualified(schema, table)+" ADD COLUMN "+column); err != nil {
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

func execDDL(rc *plugin.RequestContext, sqlText string) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	if _, err := s.pool.Exec(rc.Ctx, sqlText); err != nil {
		return nil, pgErr(err)
	}
	return actionResult{OK: true}, nil
}

func cancelQuery(rc *plugin.RequestContext) (any, error) {
	s, err := pgSession(rc)
	if err != nil {
		return nil, err
	}
	return actionResult{OK: s.cancelAll()}, nil
}

func queryStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := pgSession(rc)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(client)
	enc := json.NewEncoder(client)
	for {
		var req sqldb.QueryRequest
		if err := dec.Decode(&req); err != nil {
			if client.Context().Err() != nil {
				return nil
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err := enc.Encode(map[string]any{"error": "Invalid query request."}); err != nil {
				return err
			}
			continue
		}
		statements := sqldb.SplitStatements(req.Query)
		result, err := executeQueryRequest(client.Context(), s, req)
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

func executeQueryRequest(parent context.Context, s *Session, req sqldb.QueryRequest) (sqldb.QueryResult, error) {
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
		res, err := executeStatement(ctx, s, st)
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

func executeStatement(ctx context.Context, s *Session, statement string) (sqldb.StatementResult, error) {
	start := time.Now()
	rows, err := s.pool.Query(ctx, statement)
	if err != nil {
		return sqldb.StatementResult{}, pgErr(err)
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	out := sqldb.StatementResult{Statement: statement, Columns: fieldNames(fields)}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return sqldb.StatementResult{}, pgErr(err)
		}
		out.Rows = append(out.Rows, jsonValues(values))
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

func queryRows(ctx context.Context, s *Session, sqlText string, args []any) ([]row, error) {
	ctx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()
	rows, err := s.pool.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, pgErr(err)
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	names := fieldNames(fields)
	out := []row{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, pgErr(err)
		}
		r := row{}
		for i, name := range names {
			if i < len(values) {
				r[name] = jsonValue(values[i])
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

func jsonValues(values []any) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = jsonValue(v)
	}
	return out
}

func jsonValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	default:
		return x
	}
}

func pageRows(rc *plugin.RequestContext, rows []row) (plugin.Page[row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[row]{}, err
	}
	rows = filterRows(rows, req.Filter["q"])
	sortRows(rows, req.Sort)
	total := len(rows)
	start, err := cursorOffset(req.Cursor)
	if err != nil {
		return plugin.Page[row]{}, err
	}
	if start > len(rows) {
		start = len(rows)
	}
	end := min(start+req.Limit, len(rows))
	next := ""
	if end < len(rows) {
		next = strconv.Itoa(end)
	}
	return plugin.Page[row]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

func treeFromPage(rc *plugin.RequestContext, kind string, iconName string, labelKey string, load func(*plugin.RequestContext) (any, error)) (any, error) {
	res, err := load(rc)
	if err != nil {
		return nil, err
	}
	page, ok := res.(plugin.Page[row])
	if !ok {
		return nil, fmt.Errorf("%w: tree source returned invalid page", plugin.ErrUnavailable)
	}
	nodes := make([]plugin.TreeNode, 0, len(page.Items))
	for _, r := range page.Items {
		ref, _ := r["ref"].(plugin.ResourceRef)
		if ref.Kind == "" {
			continue
		}
		label := fmt.Sprint(r[labelKey])
		if schema := fmt.Sprint(r["schema"]); schema != "" && schema != "<nil>" && kind != "database" {
			label = schema + "." + label
		}
		nodes = append(nodes, plugin.TreeNode{
			Key:   kind + ":" + ref.UID,
			Label: label,
			Icon:  icon(iconName),
			Ref:   &ref,
			Leaf:  true,
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func filterRows(rows []row, q string) []row {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		for _, v := range r {
			if strings.Contains(strings.ToLower(fmt.Sprint(v)), q) {
				out = append(out, r)
				break
			}
		}
	}
	return out
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
	if err == pgx.ErrNoRows {
		return plugin.ErrNotFound
	}
	return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
}

func tableIdent(rc *plugin.RequestContext) (string, string, error) {
	schema, err := sqldb.SafeIdentifier(rc.Param("schema"))
	if err != nil {
		return "", "", err
	}
	table, err := sqldb.SafeIdentifier(rc.Param("table"))
	if err != nil {
		return "", "", err
	}
	return schema, table, nil
}

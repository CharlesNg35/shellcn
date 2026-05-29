package mysql

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

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

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

var dialect = sqldb.Dialect{QuoteIdent: quoteIdent, Placeholder: sqldb.QuestionPlaceholder}

func routes() []plugin.Route {
	return []plugin.Route{
		{ID: "mysql.databases.tree", Method: plugin.MethodGet, Path: "/tree/databases", Permission: "mysql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.databases.tree", Handle: treeDatabases},
		{ID: "mysql.relations.tree", Method: plugin.MethodGet, Path: "/tree/relations", Permission: "mysql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.relations.tree", Handle: treeRelations},
		{ID: "mysql.databases.list", Method: plugin.MethodGet, Path: "/databases", Permission: "mysql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.databases.list", Handle: listDatabases},
		{ID: "mysql.database.overview", Method: plugin.MethodGet, Path: "/databases/{database}/overview", Permission: "mysql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.database.overview", Handle: databaseOverview},
		{ID: "mysql.tables.list", Method: plugin.MethodGet, Path: "/tables", Permission: "mysql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.tables.list", Handle: listTables},
		{ID: "mysql.relations.graph", Method: plugin.MethodGet, Path: "/relations/graph", Permission: "mysql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.relations.graph", Handle: relationGraph},
		{ID: "mysql.views.list", Method: plugin.MethodGet, Path: "/views", Permission: "mysql.views.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.views.list", Handle: listViews},
		{ID: "mysql.view.drop", Method: plugin.MethodDelete, Path: "/views/{database}/{view}", Permission: "mysql.views.delete", Risk: plugin.RiskDestructive, AuditEvent: "mysql.view.drop", Handle: dropView},
		{ID: "mysql.routines.list", Method: plugin.MethodGet, Path: "/routines", Permission: "mysql.routines.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.routines.list", Handle: listRoutines},
		{ID: "mysql.users.tree", Method: plugin.MethodGet, Path: "/tree/users", Permission: "mysql.users.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.users.tree", Handle: treeUsers},
		{ID: "mysql.users.list", Method: plugin.MethodGet, Path: "/users", Permission: "mysql.users.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.users.list", Handle: listUsers},
		{ID: "mysql.user.overview", Method: plugin.MethodGet, Path: "/users/{host}/{user}/overview", Permission: "mysql.users.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.user.overview", Handle: userOverview},
		{ID: "mysql.table.rows", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/rows", Permission: "mysql.tables.data.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.table.rows", Handle: tableRows},
		{ID: "mysql.view.rows", Method: plugin.MethodGet, Path: "/views/{database}/{table}/rows", Permission: "mysql.views.data.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.view.rows", Handle: tableRows},
		{ID: "mysql.table.columns", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/columns", Permission: "mysql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.table.columns", Handle: tableColumnsRoute},
		{ID: "mysql.table.indexes", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/indexes", Permission: "mysql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.table.indexes", Handle: tableIndexes},
		{ID: "mysql.table.constraints", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/constraints", Permission: "mysql.tables.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.table.constraints", Handle: tableConstraints},
		{ID: "mysql.view.definition", Method: plugin.MethodGet, Path: "/views/{database}/{table}/definition", Permission: "mysql.views.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.view.definition", Handle: viewDefinition},
		{ID: "mysql.routine.definition", Method: plugin.MethodGet, Path: "/routines/{id}/definition", Permission: "mysql.routines.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.routine.definition", Handle: routineDefinition},
		{ID: "mysql.completion", Method: plugin.MethodGet, Path: "/completion", Permission: "mysql.databases.read", Risk: plugin.RiskSafe, AuditEvent: "mysql.completion", Handle: completionRoute},
		{ID: "mysql.table.row.insert", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/rows", Permission: "mysql.tables.data.write", Risk: plugin.RiskWrite, AuditEvent: "mysql.table.row.insert", Handle: insertRow},
		{ID: "mysql.table.row.update", Method: plugin.MethodPatch, Path: "/tables/{database}/{table}/rows", Permission: "mysql.tables.data.write", Risk: plugin.RiskWrite, AuditEvent: "mysql.table.row.update", Handle: updateRow},
		{ID: "mysql.table.row.delete", Method: plugin.MethodDelete, Path: "/tables/{database}/{table}/rows", Permission: "mysql.tables.data.delete", Risk: plugin.RiskDestructive, AuditEvent: "mysql.table.row.delete", Handle: deleteRow},
		{ID: "mysql.database.create", Method: plugin.MethodPost, Path: "/databases", Permission: "mysql.databases.write", Risk: plugin.RiskWrite, AuditEvent: "mysql.database.create", Input: databaseCreateSchema(), Handle: createDatabase},
		{ID: "mysql.table.create", Method: plugin.MethodPost, Path: "/databases/{database}/tables", Permission: "mysql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "mysql.table.create", Input: tableCreateSchema(), Handle: createTable},
		{ID: "mysql.column.add", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/columns", Permission: "mysql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "mysql.column.add", Input: columnAddSchema(), Handle: addColumn},
		{ID: "mysql.column.drop", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/columns/drop", Permission: "mysql.tables.write", Risk: plugin.RiskDestructive, AuditEvent: "mysql.column.drop", Handle: dropColumn},
		{ID: "mysql.index.create", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/indexes", Permission: "mysql.tables.write", Risk: plugin.RiskWrite, AuditEvent: "mysql.index.create", Input: indexCreateSchema(), Handle: createIndex},
		{ID: "mysql.index.drop", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/indexes/drop", Permission: "mysql.tables.write", Risk: plugin.RiskDestructive, AuditEvent: "mysql.index.drop", Handle: dropIndex},
		{ID: "mysql.table.truncate", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/truncate", Permission: "mysql.tables.delete", Risk: plugin.RiskDestructive, AuditEvent: "mysql.table.truncate", Handle: truncateTable},
		{ID: "mysql.table.drop", Method: plugin.MethodDelete, Path: "/tables/{database}/{table}", Permission: "mysql.tables.delete", Risk: plugin.RiskDestructive, AuditEvent: "mysql.table.drop", Handle: dropTable},
		{ID: "mysql.query", Method: plugin.MethodWS, Path: "/query", Permission: "mysql.query.execute", Risk: plugin.RiskPrivileged, AuditEvent: "mysql.query", Stream: queryStream},
		{ID: "mysql.query.cancel", Method: plugin.MethodPost, Path: "/query/cancel", Permission: "mysql.query.cancel", Risk: plugin.RiskWrite, AuditEvent: "mysql.query.cancel", Handle: cancelQuery},
	}
}

func mysqlSession(rc *plugin.RequestContext) (*Session, error) {
	return unwrap(rc.Session)
}

func databaseCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Database", Fields: []plugin.Field{
		{Key: "name", Label: "Database name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "charset", Label: "Charset", Type: plugin.FieldText, Default: "utf8mb4"},
		{Key: "collation", Label: "Collation", Type: plugin.FieldText},
		{Key: "if_not_exists", Label: "If not exists", Type: plugin.FieldToggle, Default: true},
	}}}}
}

func tableCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Table", Fields: []plugin.Field{
		{Key: "name", Label: "Table name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "columns", Label: "Columns", Type: plugin.FieldJSON, Required: true, Help: `Array of {"name":"id","type":"bigint unsigned auto_increment","primary":true,"nullable":false}`},
		{Key: "if_not_exists", Label: "If not exists", Type: plugin.FieldToggle, Default: true},
		{Key: "engine", Label: "Engine", Type: plugin.FieldText, Default: "InnoDB"},
	}}}}
}

func columnAddSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Column", Fields: []plugin.Field{
		{Key: "name", Label: "Column name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "type", Label: "Type", Type: plugin.FieldText, Required: true, Default: "varchar(255)"},
		{Key: "nullable", Label: "Nullable", Type: plugin.FieldToggle, Default: true},
		{Key: "default", Label: "Default expression", Type: plugin.FieldText},
	}}}}
}

func indexCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Index", Fields: []plugin.Field{
		{Key: "name", Label: "Index name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "columns", Label: "Columns", Type: plugin.FieldMultiSelect, Required: true, OptionsSource: &plugin.DataSource{RouteID: "mysql.table.columns", Params: tableParams()}},
		{Key: "unique", Label: "Unique", Type: plugin.FieldToggle},
	}}}}
}

// treeDatabases lists databases as expandable branches; each drills into its
// tables/views via mysql.relations.tree (hierarchical, TablePlus-style).
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
			ChildrenSource: &plugin.DataSource{RouteID: "mysql.relations.tree", Params: map[string]string{"database": name}},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

// treeRelations lists a database's tables and views as leaves (scoped by the
// p.database param the parent node supplies).
func treeRelations(rc *plugin.RequestContext) (any, error) {
	tables, err := listTables(rc)
	if err != nil {
		return nil, err
	}
	views, err := listViews(rc)
	if err != nil {
		return nil, err
	}
	nodes := []plugin.TreeNode{}
	add := func(res any, iconName string) {
		for _, r := range res.(plugin.Page[row]).Items {
			ref, ok := r["ref"].(plugin.ResourceRef)
			if !ok || ref.Kind == "" {
				continue
			}
			nodes = append(nodes, plugin.TreeNode{Key: ref.Kind + ":" + ref.UID, Label: fmt.Sprint(r["name"]), Icon: icon(iconName), Ref: &ref, Leaf: true})
		}
	}
	add(tables, "table-2")
	add(views, "panel-top")
	total := len(nodes)
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: &total}, nil
}

func treeUsers(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "user", "user", "user", listUsers)
}

func listDatabases(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT s.SCHEMA_NAME AS name, s.DEFAULT_CHARACTER_SET_NAME AS charset, s.DEFAULT_COLLATION_NAME AS collation,
       COALESCE(t.tables, 0) AS tables, COALESCE(v.views, 0) AS views
FROM information_schema.SCHEMATA s
LEFT JOIN (
  SELECT TABLE_SCHEMA, COUNT(*) AS tables
  FROM information_schema.TABLES
  WHERE TABLE_TYPE = 'BASE TABLE'
  GROUP BY TABLE_SCHEMA
) t ON t.TABLE_SCHEMA = s.SCHEMA_NAME
LEFT JOIN (
  SELECT TABLE_SCHEMA, COUNT(*) AS views
  FROM information_schema.TABLES
  WHERE TABLE_TYPE = 'VIEW'
  GROUP BY TABLE_SCHEMA
) v ON v.TABLE_SCHEMA = s.SCHEMA_NAME
WHERE s.SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'sys')
ORDER BY s.SCHEMA_NAME`, nil)
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
	database, err := sqldb.SafeIdentifier(rc.Param("database"))
	if err != nil {
		return nil, err
	}
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT s.SCHEMA_NAME AS name, s.DEFAULT_CHARACTER_SET_NAME AS charset, s.DEFAULT_COLLATION_NAME AS collation,
       VERSION() AS version,
       COALESCE(SUM(t.DATA_LENGTH + t.INDEX_LENGTH), 0) AS size,
       COUNT(CASE WHEN t.TABLE_TYPE = 'BASE TABLE' THEN 1 END) AS tables,
       COUNT(CASE WHEN t.TABLE_TYPE = 'VIEW' THEN 1 END) AS views
FROM information_schema.SCHEMATA s
LEFT JOIN information_schema.TABLES t ON t.TABLE_SCHEMA = s.SCHEMA_NAME
WHERE s.SCHEMA_NAME = ?
GROUP BY s.SCHEMA_NAME, s.DEFAULT_CHARACTER_SET_NAME, s.DEFAULT_COLLATION_NAME`, []any{database})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

func listTables(rc *plugin.RequestContext) (any, error) {
	return relationList(rc, "BASE TABLE", "table")
}

const relationGraphSQL = `
SELECT CONSTRAINT_NAME AS constraint_name,
       TABLE_SCHEMA AS child_schema, TABLE_NAME AS child_table, COLUMN_NAME AS child_column,
       REFERENCED_TABLE_SCHEMA AS parent_schema, REFERENCED_TABLE_NAME AS parent_table, REFERENCED_COLUMN_NAME AS parent_column
FROM information_schema.KEY_COLUMN_USAGE
WHERE REFERENCED_TABLE_NAME IS NOT NULL
  AND TABLE_SCHEMA NOT IN ('information_schema', 'performance_schema', 'sys', 'mysql')
  AND (? = '' OR TABLE_SCHEMA = ?)
ORDER BY CONSTRAINT_NAME, ORDINAL_POSITION`

const relationColumnsSQL = `
SELECT TABLE_SCHEMA AS table_schema, TABLE_NAME AS table_name, COLUMN_NAME AS column_name, COLUMN_TYPE AS data_type
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA NOT IN ('information_schema', 'performance_schema', 'sys', 'mysql')
  AND (? = '' OR TABLE_SCHEMA = ?)
ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`

func relationGraph(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	database, err := sqldb.OptionalIdentifier(rc.Query().Get("p.database"))
	if err != nil {
		return nil, err
	}
	colRows, err := queryRows(rc.Ctx, s, relationColumnsSQL, []any{database, database})
	if err != nil {
		return nil, err
	}
	fkRows, err := queryRows(rc.Ctx, s, relationGraphSQL, []any{database, database})
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
	return relationList(rc, "VIEW", "view")
}

func relationList(rc *plugin.RequestContext, tableType string, refKind string) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	database, err := sqldb.OptionalIdentifier(rc.Query().Get("p.database"))
	if err != nil {
		return nil, err
	}
	sqlText := `
SELECT TABLE_NAME AS name, TABLE_SCHEMA AS ` + "`database`" + `, ENGINE AS engine,
       TABLE_ROWS AS ` + "`rows`" + `, COALESCE(DATA_LENGTH, 0) + COALESCE(INDEX_LENGTH, 0) AS size,
       TABLE_COLLATION AS collation, NULL AS definer, NULL AS updatable
FROM information_schema.TABLES
WHERE TABLE_TYPE = ? AND TABLE_SCHEMA NOT IN ('information_schema', 'performance_schema', 'sys')
  AND (? = '' OR TABLE_SCHEMA = ?)
ORDER BY TABLE_SCHEMA, TABLE_NAME`
	if tableType == "VIEW" {
		sqlText = `
SELECT t.TABLE_NAME AS name, t.TABLE_SCHEMA AS ` + "`database`" + `, t.ENGINE AS engine,
       t.TABLE_ROWS AS ` + "`rows`" + `, COALESCE(t.DATA_LENGTH, 0) + COALESCE(t.INDEX_LENGTH, 0) AS size,
       t.TABLE_COLLATION AS collation, v.DEFINER AS definer,
       CASE WHEN v.IS_UPDATABLE = 'YES' THEN true ELSE false END AS updatable
FROM information_schema.TABLES t
JOIN information_schema.VIEWS v ON v.TABLE_SCHEMA = t.TABLE_SCHEMA AND v.TABLE_NAME = t.TABLE_NAME
WHERE t.TABLE_TYPE = ? AND t.TABLE_SCHEMA NOT IN ('information_schema', 'performance_schema', 'sys')
  AND (? = '' OR t.TABLE_SCHEMA = ?)
ORDER BY t.TABLE_SCHEMA, t.TABLE_NAME`
	}
	rows, err := queryRows(rc.Ctx, s, sqlText, []any{tableType, database, database})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name, database := fmt.Sprint(r["name"]), fmt.Sprint(r["database"])
		r["ref"] = plugin.ResourceRef{Kind: refKind, Namespace: database, Name: name, UID: database + "." + name}
	}
	return pageRows(rc, rows)
}

func listRoutines(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	database, err := sqldb.OptionalIdentifier(rc.Query().Get("p.database"))
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT ROUTINE_NAME AS name, ROUTINE_SCHEMA AS `+"`database`"+`, ROUTINE_TYPE AS `+"`type`"+`,
       DATA_TYPE AS `+"`returns`"+`, DEFINER AS definer, LAST_ALTERED AS modified
FROM information_schema.ROUTINES
WHERE ROUTINE_SCHEMA NOT IN ('information_schema', 'performance_schema', 'sys')
  AND (? = '' OR ROUTINE_SCHEMA = ?)
ORDER BY ROUTINE_SCHEMA, ROUTINE_NAME`, []any{database, database})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name, database, routineType := fmt.Sprint(r["name"]), fmt.Sprint(r["database"]), strings.ToUpper(fmt.Sprint(r["type"]))
		r["ref"] = plugin.ResourceRef{Kind: "routine", Namespace: database, Name: name, UID: routineID(database, routineType, name)}
	}
	return pageRows(rc, rows)
}

func listUsers(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT User AS `+"`user`"+`, Host AS host, plugin, false AS locked
FROM mysql.user
ORDER BY User, Host`, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		user, host := fmt.Sprint(r["user"]), fmt.Sprint(r["host"])
		r["ref"] = plugin.ResourceRef{Kind: "user", Namespace: host, Name: user, UID: user + "@" + host}
	}
	return pageRows(rc, rows)
}

func userOverview(rc *plugin.RequestContext) (any, error) {
	user := strings.TrimSpace(rc.Param("user"))
	host := strings.TrimSpace(rc.Param("host"))
	if user == "" || host == "" {
		return nil, fmt.Errorf("%w: user and host are required", plugin.ErrInvalidInput)
	}
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT User AS `+"`user`"+`, Host AS host, plugin, false AS locked
FROM mysql.user
WHERE User = ? AND Host = ?`, []any{user, host})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

func tableRows(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	s, err := mysqlSession(rc)
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
	var total int
	if err := s.db.QueryRowContext(rc.Ctx, "SELECT COUNT(*) FROM "+qualified(database, table)).Scan(&total); err != nil {
		return nil, mysqlErr(err)
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
		orderBy = " ORDER BY " + quoteIdent(col) + " " + dir
	}
	rows, err := queryRows(rc.Ctx, s, fmt.Sprintf("SELECT * FROM %s%s LIMIT ? OFFSET ?", qualified(database, table), orderBy), []any{limit, offset})
	if err != nil {
		return nil, err
	}
	pk, err := primaryKeyColumns(rc.Ctx, s, database, table)
	if err != nil {
		return nil, err
	}
	attachRowKeys(rows, pk, s.opts.RedactPatterns)
	fks, err := foreignKeys(rc.Ctx, s, database, table)
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

// foreignKeys maps each FK column to the referenced table's ref, attached under
// the generic "_links" field the grid renders as links.
func foreignKeys(ctx context.Context, s *Session, database, table string) (map[string]plugin.ResourceRef, error) {
	rows, err := queryRows(ctx, s, `
SELECT COLUMN_NAME AS col, REFERENCED_TABLE_SCHEMA AS ref_schema, REFERENCED_TABLE_NAME AS ref_table
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL`, []any{database, table})
	if err != nil {
		return nil, err
	}
	out := map[string]plugin.ResourceRef{}
	for _, r := range rows {
		col, refSchema, refTable := fmt.Sprint(r["col"]), fmt.Sprint(r["ref_schema"]), fmt.Sprint(r["ref_table"])
		out[col] = plugin.ResourceRef{Kind: "table", Namespace: refSchema, Name: refTable, UID: refSchema + "." + refTable}
	}
	return out, nil
}

func attachForeignKeys(rows []row, fks map[string]plugin.ResourceRef) {
	if len(fks) == 0 {
		return
	}
	for _, r := range rows {
		r["_links"] = fks
	}
}

func tableColumnsRoute(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT COLUMN_NAME AS name, COLUMN_TYPE AS type, IS_NULLABLE = 'YES' AS nullable,
       COLUMN_DEFAULT AS `+"`default`"+`, EXTRA AS extra, ORDINAL_POSITION AS position
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION`, []any{database, table})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		name := fmt.Sprint(rows[i]["name"])
		rows[i]["ref"] = plugin.ResourceRef{Kind: "column", Scope: database, Namespace: table, Name: name, UID: database + "." + table + "." + name}
	}
	return pageRows(rc, rows)
}

func tableIndexes(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT INDEX_NAME AS name, COLUMN_NAME AS `+"`column`"+`,
       NON_UNIQUE = 0 AS `+"`unique`"+`, INDEX_TYPE AS type, SEQ_IN_INDEX AS sequence
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY INDEX_NAME, SEQ_IN_INDEX`, []any{database, table})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		name := fmt.Sprint(rows[i]["name"])
		rows[i]["ref"] = plugin.ResourceRef{Kind: "index", Scope: database, Namespace: table, Name: name, UID: database + "." + table + "." + name + "." + fmt.Sprint(rows[i]["column"])}
	}
	return pageRows(rc, rows)
}

func tableConstraints(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT tc.CONSTRAINT_NAME AS name, tc.CONSTRAINT_TYPE AS type, kcu.COLUMN_NAME AS `+"`column`"+`,
       CONCAT_WS('.', kcu.REFERENCED_TABLE_SCHEMA, kcu.REFERENCED_TABLE_NAME, kcu.REFERENCED_COLUMN_NAME) AS referenced
FROM information_schema.TABLE_CONSTRAINTS tc
LEFT JOIN information_schema.KEY_COLUMN_USAGE kcu
  ON kcu.CONSTRAINT_SCHEMA = tc.CONSTRAINT_SCHEMA
 AND kcu.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
 AND kcu.TABLE_SCHEMA = tc.TABLE_SCHEMA
 AND kcu.TABLE_NAME = tc.TABLE_NAME
WHERE tc.TABLE_SCHEMA = ? AND tc.TABLE_NAME = ?
ORDER BY tc.CONSTRAINT_NAME`, []any{database, table})
	if err != nil {
		return nil, err
	}
	return pageRows(rc, rows)
}

func viewDefinition(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT VIEW_DEFINITION AS definition, DEFINER AS definer, CHECK_OPTION AS checkOption
FROM information_schema.VIEWS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, []any{database, table})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

func routineDefinition(rc *plugin.RequestContext) (any, error) {
	database, routineType, routine, err := parseRoutineID(rc.Param("id"))
	if err != nil {
		return nil, err
	}
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, "SHOW CREATE "+routineType+" "+qualified(database, routine), nil)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

func completionRoute(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
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
SELECT TABLE_SCHEMA AS database_name, TABLE_NAME AS relation_name, TABLE_TYPE AS relation_type, COLUMN_NAME AS column_name
FROM information_schema.COLUMNS c
JOIN information_schema.TABLES t USING (TABLE_SCHEMA, TABLE_NAME)
WHERE TABLE_SCHEMA NOT IN ('information_schema', 'performance_schema', 'sys')
ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION
LIMIT 2500`, nil)
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
		database := fmt.Sprint(r["database_name"])
		relation := fmt.Sprint(r["relation_name"])
		kind := "table"
		if fmt.Sprint(r["relation_type"]) == "VIEW" {
			kind = "view"
		}
		add(sqldb.CompletionItem{Label: database, Type: "namespace", Detail: "database"})
		add(sqldb.CompletionItem{Label: relation, Type: kind, Detail: database, Apply: quoteIdent(database) + "." + quoteIdent(relation)})
		column := fmt.Sprint(r["column_name"])
		if column != "" && column != "<nil>" {
			add(sqldb.CompletionItem{Label: column, Type: "property", Detail: database + "." + relation})
		}
	}
	return items, nil
}

func createDatabase(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	var req struct {
		Name        string `json:"name" validate:"required"`
		Charset     string `json:"charset"`
		Collation   string `json:"collation"`
		IfNotExists bool   `json:"if_not_exists"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := sqldb.SafeIdentifier(req.Name)
	if err != nil {
		return nil, err
	}
	prefix := "CREATE DATABASE "
	if req.IfNotExists {
		prefix += "IF NOT EXISTS "
	}
	stmt := prefix + quoteIdent(name)
	if charset := strings.TrimSpace(req.Charset); charset != "" {
		if _, err := sqldb.SafeIdentifier(charset); err != nil {
			return nil, err
		}
		stmt += " CHARACTER SET " + charset
	}
	if collation := strings.TrimSpace(req.Collation); collation != "" {
		if _, err := sqldb.SafeIdentifier(collation); err != nil {
			return nil, err
		}
		stmt += " COLLATE " + collation
	}
	if _, err := s.db.ExecContext(rc.Ctx, stmt); err != nil {
		return nil, mysqlErr(err)
	}
	return actionResult{OK: true}, nil
}

func createTable(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	database, err := sqldb.SafeIdentifier(rc.Param("database"))
	if err != nil {
		return nil, err
	}
	var req struct {
		Name        string `json:"name" validate:"required"`
		Columns     any    `json:"columns" validate:"required"`
		IfNotExists bool   `json:"if_not_exists"`
		Engine      string `json:"engine"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	table, err := sqldb.SafeIdentifier(req.Name)
	if err != nil {
		return nil, err
	}
	columns, err := parseDDLColumns(req.Columns)
	if err != nil {
		return nil, err
	}
	engine := strings.TrimSpace(req.Engine)
	if engine == "" {
		engine = "InnoDB"
	}
	if !sqldb.SafeType(engine) {
		return nil, fmt.Errorf("%w: unsafe table engine", plugin.ErrInvalidInput)
	}
	prefix := "CREATE TABLE "
	if req.IfNotExists {
		prefix += "IF NOT EXISTS "
	}
	sqlText := prefix + qualified(database, table) + " (" + strings.Join(columns, ", ") + ") ENGINE=" + engine
	if _, err := s.db.ExecContext(rc.Ctx, sqlText); err != nil {
		return nil, mysqlErr(err)
	}
	return actionResult{OK: true}, nil
}

func addColumn(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	database, table, err := tableIdent(rc)
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
	column, err := ddlColumn(sqldb.ColumnSpec{Name: req.Name, Type: req.Type, Nullable: req.Nullable, Default: req.Default})
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(rc.Ctx, "ALTER TABLE "+qualified(database, table)+" ADD COLUMN "+column); err != nil {
		return nil, mysqlErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropColumn(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	column, err := sqldb.SafeIdentifier(rc.Param("name"))
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(rc.Ctx, "ALTER TABLE "+qualified(database, table)+" DROP COLUMN "+quoteIdent(column)); err != nil {
		return nil, mysqlErr(err)
	}
	return actionResult{OK: true}, nil
}

func createIndex(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	database, table, err := tableIdent(rc)
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
	cols, err := sqldb.IdentifierListValue(req.Columns, quoteIdent)
	if err != nil {
		return nil, err
	}
	unique := ""
	if req.Unique {
		unique = "UNIQUE "
	}
	stmt := "CREATE " + unique + "INDEX " + quoteIdent(name) + " ON " + qualified(database, table) + " (" + strings.Join(cols, ", ") + ")"
	if _, err := s.db.ExecContext(rc.Ctx, stmt); err != nil {
		return nil, mysqlErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropIndex(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	name, err := sqldb.SafeIdentifier(rc.Param("name"))
	if err != nil {
		return nil, err
	}
	// MySQL drops indexes relative to their table: DROP INDEX name ON db.table.
	if _, err := s.db.ExecContext(rc.Ctx, "DROP INDEX "+quoteIdent(name)+" ON "+qualified(database, table)); err != nil {
		return nil, mysqlErr(err)
	}
	return actionResult{OK: true}, nil
}

func insertRow(rc *plugin.RequestContext) (any, error) {
	s, table, m, err := rowMutationInput(rc)
	if err != nil {
		return nil, err
	}
	stmt, args, err := dialect.Insert(table, m.Values)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(rc.Ctx, stmt, args...); err != nil {
		return nil, mysqlErr(err)
	}
	return actionResult{OK: true}, nil
}

func updateRow(rc *plugin.RequestContext) (any, error) {
	return keyedRowMutation(rc, false)
}

func deleteRow(rc *plugin.RequestContext) (any, error) {
	return keyedRowMutation(rc, true)
}

func rowMutationInput(rc *plugin.RequestContext) (*Session, string, sqldb.RowMutation, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, "", sqldb.RowMutation{}, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, "", sqldb.RowMutation{}, err
	}
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, "", sqldb.RowMutation{}, err
	}
	var m sqldb.RowMutation
	if err := rc.Bind(&m); err != nil {
		return nil, "", sqldb.RowMutation{}, err
	}
	return s, qualified(database, table), m, nil
}

// keyedRowMutation runs an UPDATE or DELETE, but only after confirming the
// client's key is exactly the table's primary key and that it affects one row.
func keyedRowMutation(rc *plugin.RequestContext, del bool) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	var m sqldb.RowMutation
	if err := rc.Bind(&m); err != nil {
		return nil, err
	}
	pk, err := primaryKeyColumns(rc.Ctx, s, database, table)
	if err != nil {
		return nil, err
	}
	if err := sqldb.ValidateRowKey(pk, m.Key); err != nil {
		return nil, err
	}
	qual := qualified(database, table)
	var stmt string
	var args []any
	if del {
		stmt, args, err = dialect.Delete(qual, m.Key)
	} else {
		stmt, args, err = dialect.Update(qual, m.Key, m.Values)
	}
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(rc.Ctx, stmt, args...)
	if err != nil {
		return nil, mysqlErr(err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("%w: row no longer matches (it may have changed)", plugin.ErrNotFound)
	}
	return actionResult{OK: true}, nil
}

func primaryKeyColumns(ctx context.Context, s *Session, database, table string) ([]string, error) {
	rows, err := queryRows(ctx, s, `
SELECT COLUMN_NAME AS name
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'
ORDER BY ORDINAL_POSITION`, []any{database, table})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprint(r["name"]))
	}
	return out, nil
}

// attachRowKeys tags each row with the primary-key map the editable grid echoes
// back for UPDATE/DELETE. The grid stays read-only when the table has no primary
// key or when a key column is itself sensitive (so a redacted value is never
// shipped raw inside _key).
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

func truncateTable(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	return execDDL(rc, "TRUNCATE TABLE "+qualified(database, table))
}

func dropTable(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	return execDDL(rc, "DROP TABLE "+qualified(database, table))
}

func dropView(rc *plugin.RequestContext) (any, error) {
	database, err := sqldb.SafeIdentifier(rc.Param("database"))
	if err != nil {
		return nil, err
	}
	view, err := sqldb.SafeIdentifier(rc.Param("view"))
	if err != nil {
		return nil, err
	}
	return execDDL(rc, "DROP VIEW "+qualified(database, view))
}

func execDDL(rc *plugin.RequestContext, sqlText string) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(rc.Ctx, sqlText); err != nil {
		return nil, mysqlErr(err)
	}
	return actionResult{OK: true}, nil
}

func cancelQuery(rc *plugin.RequestContext) (any, error) {
	s, err := mysqlSession(rc)
	if err != nil {
		return nil, err
	}
	return actionResult{OK: s.cancelAll()}, nil
}

func queryStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := mysqlSession(rc)
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
				payload["confirmMessage"] = "This MySQL statement can change data, schema, or privileges. Review it before running."
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

func executeStatement(ctx context.Context, s *Session, statement string) (sqldb.StatementResult, error) {
	start := time.Now()
	if !statementReturnsRows(statement) {
		res, err := s.db.ExecContext(ctx, statement)
		if err != nil {
			return sqldb.StatementResult{}, mysqlErr(err)
		}
		affected, _ := res.RowsAffected()
		out := sqldb.StatementResult{Statement: statement, RowCount: affected, ElapsedMS: time.Since(start).Milliseconds()}
		out.CommandTag = sqldb.FirstKeyword(statement)
		if affected >= 0 {
			out.CommandTag += " " + strconv.FormatInt(affected, 10)
		}
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, statement)
	if err != nil {
		return sqldb.StatementResult{}, mysqlErr(err)
	}
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return sqldb.StatementResult{}, mysqlErr(err)
	}
	out := sqldb.StatementResult{Statement: statement, Columns: columns}
	for rows.Next() {
		values, err := scanValues(rows, columns)
		if err != nil {
			_ = rows.Close()
			return sqldb.StatementResult{}, mysqlErr(err)
		}
		out.Rows = append(out.Rows, values)
		if len(out.Rows) >= s.opts.RowLimit {
			break
		}
	}
	if err := rows.Close(); err != nil {
		return sqldb.StatementResult{}, mysqlErr(err)
	}
	if err := rows.Err(); err != nil {
		return sqldb.StatementResult{}, mysqlErr(err)
	}
	out.RowCount = int64(len(out.Rows))
	out.CommandTag = sqldb.FirstKeyword(statement)
	out.Rows = sqldb.RedactRows(out.Columns, out.Rows, s.opts.RedactPatterns)
	out.ElapsedMS = time.Since(start).Milliseconds()
	return out, nil
}

func statementReturnsRows(statement string) bool {
	switch sqldb.FirstKeyword(statement) {
	case "SELECT", "SHOW", "EXPLAIN", "WITH", "VALUES", "DESCRIBE", "DESC":
		return true
	default:
		return false
	}
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

func queryRows(ctx context.Context, s *Session, sqlText string, args []any) ([]row, error) {
	ctx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, mysqlErr(err)
	}
	defer func() { _ = rows.Close() }()
	columns, err := rows.Columns()
	if err != nil {
		return nil, mysqlErr(err)
	}
	out := []row{}
	for rows.Next() {
		values, err := scanValues(rows, columns)
		if err != nil {
			return nil, mysqlErr(err)
		}
		r := row{}
		for i, name := range columns {
			if i < len(values) {
				r[name] = values[i]
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, mysqlErr(err)
	}
	return out, nil
}

func scanValues(rows *sql.Rows, columns []string) ([]any, error) {
	values := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	values = sqldb.DisplayValues(columns, values)
	return values, nil
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
		if database := fmt.Sprint(r["database"]); database != "" && database != "<nil>" && kind != "database" {
			label = database + "." + label
		}
		if kind == "user" {
			label = fmt.Sprintf("%s@%s", r["user"], r["host"])
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
		for k, v := range r {
			if k == "_key" || k == "ref" {
				continue
			}
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

func mysqlErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return plugin.ErrNotFound
	}
	var me *mysqldriver.MySQLError
	if errors.As(err, &me) && me.Number == 1142 {
		return fmt.Errorf("%w: %v", plugin.ErrForbidden, err)
	}
	return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
}

func tableIdent(rc *plugin.RequestContext) (string, string, error) {
	database, err := sqldb.SafeIdentifier(rc.Param("database"))
	if err != nil {
		return "", "", err
	}
	table, err := sqldb.SafeIdentifier(rc.Param("table"))
	if err != nil {
		return "", "", err
	}
	return database, table, nil
}

func qualified(database, name string) string {
	return quoteIdent(database) + "." + quoteIdent(name)
}

func routineID(database, routineType, routine string) string {
	return database + ":" + strings.ToUpper(routineType) + ":" + routine
}

func parseRoutineID(id string) (string, string, string, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("%w: routine id is invalid", plugin.ErrInvalidInput)
	}
	database, err := sqldb.SafeIdentifier(parts[0])
	if err != nil {
		return "", "", "", err
	}
	routineType := strings.ToUpper(strings.TrimSpace(parts[1]))
	if routineType != "FUNCTION" && routineType != "PROCEDURE" {
		return "", "", "", fmt.Errorf("%w: routine type is invalid", plugin.ErrInvalidInput)
	}
	routine, err := sqldb.SafeIdentifier(parts[2])
	if err != nil {
		return "", "", "", err
	}
	return database, routineType, routine, nil
}

func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func parseDDLColumns(value any) ([]string, error) {
	raw, err := sqldb.NormalizeJSONValue(value)
	if err != nil {
		return nil, err
	}
	var specs []sqldb.ColumnSpec
	if err := json.Unmarshal(raw, &specs); err != nil || len(specs) == 0 {
		return nil, fmt.Errorf("%w: columns must be a non-empty JSON array", plugin.ErrInvalidInput)
	}
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		col, err := ddlColumn(spec)
		if err != nil {
			return nil, err
		}
		out = append(out, col)
	}
	return out, nil
}

func ddlColumn(spec sqldb.ColumnSpec) (string, error) {
	name, err := sqldb.SafeIdentifier(spec.Name)
	if err != nil {
		return "", err
	}
	dataType := strings.TrimSpace(spec.Type)
	if !sqldb.SafeType(dataType) {
		return "", fmt.Errorf("%w: unsafe column type", plugin.ErrInvalidInput)
	}
	parts := []string{quoteIdent(name), dataType}
	if !spec.Nullable || spec.Primary {
		parts = append(parts, "NOT NULL")
	}
	if strings.TrimSpace(spec.Default) != "" {
		if !sqldb.SafeDefault(spec.Default) {
			return "", fmt.Errorf("%w: unsafe default expression", plugin.ErrInvalidInput)
		}
		parts = append(parts, "DEFAULT "+strings.TrimSpace(spec.Default))
	}
	if spec.Primary {
		parts = append(parts, "PRIMARY KEY")
	}
	if spec.Unique {
		parts = append(parts, "UNIQUE")
	}
	return strings.Join(parts, " "), nil
}

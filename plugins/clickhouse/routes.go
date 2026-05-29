package clickhouse

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

func routes() []plugin.Route {
	return []plugin.Route{
		{ID: "clickhouse.databases.tree", Method: plugin.MethodGet, Path: "/tree/databases", Permission: "clickhouse.databases.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.databases.tree", Handle: treeDatabases},
		{ID: "clickhouse.databases.list", Method: plugin.MethodGet, Path: "/databases", Permission: "clickhouse.databases.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.databases.list", Handle: listDatabases},
		{ID: "clickhouse.database.overview", Method: plugin.MethodGet, Path: "/databases/{database}/overview", Permission: "clickhouse.databases.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.database.overview", Handle: databaseOverview},
		{ID: "clickhouse.relations.tree", Method: plugin.MethodGet, Path: "/tree/relations", Permission: "clickhouse.tables.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.relations.tree", Handle: treeRelations},
		{ID: "clickhouse.tables.list", Method: plugin.MethodGet, Path: "/tables", Permission: "clickhouse.tables.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.tables.list", Handle: listTables},
		{ID: "clickhouse.views.list", Method: plugin.MethodGet, Path: "/views", Permission: "clickhouse.views.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.views.list", Handle: listViews},
		{ID: "clickhouse.view.drop", Method: plugin.MethodDelete, Path: "/views/{database}/{view}", Permission: "clickhouse.tables.write", Risk: plugin.RiskDestructive, AuditEvent: "clickhouse.view.drop", Handle: dropView},
		{ID: "clickhouse.dictionaries.tree", Method: plugin.MethodGet, Path: "/tree/dictionaries", Permission: "clickhouse.dictionaries.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.dictionaries.tree", Handle: treeDictionaries},
		{ID: "clickhouse.dictionaries.list", Method: plugin.MethodGet, Path: "/dictionaries", Permission: "clickhouse.dictionaries.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.dictionaries.list", Handle: listDictionaries},
		{ID: "clickhouse.dictionary.overview", Method: plugin.MethodGet, Path: "/dictionaries/{database}/{table}/overview", Permission: "clickhouse.dictionaries.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.dictionary.overview", Handle: dictionaryOverview},
		{ID: "clickhouse.mutations.tree", Method: plugin.MethodGet, Path: "/tree/mutations", Permission: "clickhouse.mutations.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.mutations.tree", Handle: treeMutations},
		{ID: "clickhouse.mutations.list", Method: plugin.MethodGet, Path: "/mutations", Permission: "clickhouse.mutations.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.mutations.list", Handle: listMutations},
		{ID: "clickhouse.mutation.overview", Method: plugin.MethodGet, Path: "/mutations/{id}/overview", Permission: "clickhouse.mutations.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.mutation.overview", Handle: mutationOverview},
		{ID: "clickhouse.merges.tree", Method: plugin.MethodGet, Path: "/tree/merges", Permission: "clickhouse.merges.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.merges.tree", Handle: treeMerges},
		{ID: "clickhouse.merges.list", Method: plugin.MethodGet, Path: "/merges", Permission: "clickhouse.merges.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.merges.list", Handle: listMerges},
		{ID: "clickhouse.merge.overview", Method: plugin.MethodGet, Path: "/merges/{id}/overview", Permission: "clickhouse.merges.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.merge.overview", Handle: mergeOverview},
		{ID: "clickhouse.processes.tree", Method: plugin.MethodGet, Path: "/tree/processes", Permission: "clickhouse.processes.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.processes.tree", Handle: treeProcesses},
		{ID: "clickhouse.processes.list", Method: plugin.MethodGet, Path: "/processes", Permission: "clickhouse.processes.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.processes.list", Handle: listProcesses},
		{ID: "clickhouse.process.overview", Method: plugin.MethodGet, Path: "/processes/{id}/overview", Permission: "clickhouse.processes.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.process.overview", Handle: processOverview},
		{ID: "clickhouse.users.tree", Method: plugin.MethodGet, Path: "/tree/users", Permission: "clickhouse.users.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.users.tree", Handle: treeUsers},
		{ID: "clickhouse.users.list", Method: plugin.MethodGet, Path: "/users", Permission: "clickhouse.users.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.users.list", Handle: listUsers},
		{ID: "clickhouse.user.overview", Method: plugin.MethodGet, Path: "/users/{user}/overview", Permission: "clickhouse.users.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.user.overview", Handle: userOverview},
		{ID: "clickhouse.table.rows", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/rows", Permission: "clickhouse.tables.data.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.table.rows", Handle: tableRows},
		{ID: "clickhouse.view.rows", Method: plugin.MethodGet, Path: "/views/{database}/{table}/rows", Permission: "clickhouse.views.data.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.view.rows", Handle: tableRows},
		{ID: "clickhouse.table.columns", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/columns", Permission: "clickhouse.tables.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.table.columns", Handle: tableColumnsRoute},
		{ID: "clickhouse.table.indexes", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/indexes", Permission: "clickhouse.tables.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.table.indexes", Handle: tableIndexes},
		{ID: "clickhouse.table.constraints", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/constraints", Permission: "clickhouse.tables.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.table.constraints", Handle: tableConstraints},
		{ID: "clickhouse.table.definition", Method: plugin.MethodGet, Path: "/tables/{database}/{table}/definition", Permission: "clickhouse.tables.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.table.definition", Handle: tableDefinition},
		{ID: "clickhouse.view.definition", Method: plugin.MethodGet, Path: "/views/{database}/{table}/definition", Permission: "clickhouse.views.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.view.definition", Handle: tableDefinition},
		{ID: "clickhouse.completion", Method: plugin.MethodGet, Path: "/completion", Permission: "clickhouse.databases.read", Risk: plugin.RiskSafe, AuditEvent: "clickhouse.completion", Handle: completionRoute},
		{ID: "clickhouse.database.create", Method: plugin.MethodPost, Path: "/databases", Permission: "clickhouse.databases.write", Risk: plugin.RiskWrite, AuditEvent: "clickhouse.database.create", Input: databaseCreateSchema(), Handle: createDatabase},
		{ID: "clickhouse.table.create", Method: plugin.MethodPost, Path: "/databases/{database}/tables", Permission: "clickhouse.tables.write", Risk: plugin.RiskWrite, AuditEvent: "clickhouse.table.create", Input: tableCreateSchema(), Handle: createTable},
		{ID: "clickhouse.column.add", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/columns", Permission: "clickhouse.tables.write", Risk: plugin.RiskWrite, AuditEvent: "clickhouse.column.add", Input: columnAddSchema(), Handle: addColumn},
		{ID: "clickhouse.column.drop", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/columns/drop", Permission: "clickhouse.tables.write", Risk: plugin.RiskDestructive, AuditEvent: "clickhouse.column.drop", Handle: dropColumn},
		{ID: "clickhouse.index.drop", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/indexes/drop", Permission: "clickhouse.tables.write", Risk: plugin.RiskDestructive, AuditEvent: "clickhouse.index.drop", Handle: dropIndex},
		{ID: "clickhouse.table.truncate", Method: plugin.MethodPost, Path: "/tables/{database}/{table}/truncate", Permission: "clickhouse.tables.delete", Risk: plugin.RiskDestructive, AuditEvent: "clickhouse.table.truncate", Handle: truncateTable},
		{ID: "clickhouse.table.drop", Method: plugin.MethodDelete, Path: "/tables/{database}/{table}", Permission: "clickhouse.tables.delete", Risk: plugin.RiskDestructive, AuditEvent: "clickhouse.table.drop", Handle: dropTable},
		{ID: "clickhouse.query", Method: plugin.MethodWS, Path: "/query", Permission: "clickhouse.query.execute", Risk: plugin.RiskPrivileged, AuditEvent: "clickhouse.query", Stream: queryStream},
		{ID: "clickhouse.query.cancel", Method: plugin.MethodPost, Path: "/query/cancel", Permission: "clickhouse.query.cancel", Risk: plugin.RiskWrite, AuditEvent: "clickhouse.query.cancel", Handle: cancelQuery},
	}
}

func clickhouseSession(rc *plugin.RequestContext) (*Session, error) {
	return unwrap(rc.Session)
}

func databaseCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Database", Fields: []plugin.Field{
		{Key: "name", Label: "Database name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "if_not_exists", Label: "If not exists", Type: plugin.FieldToggle, Default: true},
	}}}}
}

func tableCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Table", Fields: []plugin.Field{
		{Key: "name", Label: "Table name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "columns", Label: "Columns", Type: plugin.FieldJSON, Required: true, Help: `Array of {"name":"event_time","type":"DateTime","nullable":false}`},
		{Key: "engine", Label: "Engine", Type: plugin.FieldText, Required: true, Default: "MergeTree"},
		{Key: "order_by", Label: "ORDER BY", Type: plugin.FieldText, Required: true, Default: "tuple()"},
		{Key: "if_not_exists", Label: "If not exists", Type: plugin.FieldToggle, Default: true},
	}}}}
}

func columnAddSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Column", Fields: []plugin.Field{
		{Key: "name", Label: "Column name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: sqldb.IdentifierPattern}}},
		{Key: "type", Label: "Type", Type: plugin.FieldText, Required: true, Default: "String"},
		{Key: "nullable", Label: "Nullable", Type: plugin.FieldToggle, Default: false},
		{Key: "default", Label: "Default expression", Type: plugin.FieldText},
	}}}}
}

// treeDatabases lists databases as expandable branches that drill into their
// tables/views (hierarchical, TablePlus-style).
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
			ChildrenSource: &plugin.DataSource{RouteID: "clickhouse.relations.tree", Params: map[string]string{"database": name}},
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

func treeDictionaries(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "dictionary", "book-open", "name", listDictionaries)
}

func treeMutations(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "mutation", "git-compare-arrows", "mutation_id", listMutations)
}

func treeMerges(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "merge", "merge", "id", listMerges)
}

func treeProcesses(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "process", "activity", "query_id", listProcesses)
}

func treeUsers(rc *plugin.RequestContext) (any, error) {
	return treeFromPage(rc, "user", "user", "user", listUsers)
}

func listDatabases(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT d.name, d.engine, d.comment,
       countIf(t.engine NOT IN ('View', 'MaterializedView', 'LiveView', 'WindowView')) AS tables,
       countIf(t.engine IN ('View', 'MaterializedView', 'LiveView', 'WindowView')) AS views,
       sum(ifNull(t.total_bytes, 0)) AS size
FROM system.databases d
LEFT JOIN system.tables t ON t.database = d.name
WHERE d.name NOT IN ('INFORMATION_SCHEMA', 'information_schema', 'system')
GROUP BY d.name, d.engine, d.comment
ORDER BY d.name`, nil)
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
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT d.name, d.engine, d.comment, version() AS version,
       countIf(t.engine NOT IN ('View', 'MaterializedView', 'LiveView', 'WindowView')) AS tables,
       countIf(t.engine IN ('View', 'MaterializedView', 'LiveView', 'WindowView')) AS views,
       sum(ifNull(t.total_rows, 0)) AS rows,
       sum(ifNull(t.total_bytes, 0)) AS size
FROM system.databases d
LEFT JOIN system.tables t ON t.database = d.name
WHERE d.name = ?
GROUP BY d.name, d.engine, d.comment`, []any{database})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

func listTables(rc *plugin.RequestContext) (any, error) {
	return relationList(rc, false, "table")
}

func listViews(rc *plugin.RequestContext) (any, error) {
	return relationList(rc, true, "view")
}

func relationList(rc *plugin.RequestContext, views bool, refKind string) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	database, table, err := optionalTableFilter(rc)
	if err != nil {
		return nil, err
	}
	op := "NOT IN"
	if views {
		op = "IN"
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT name, database, engine, ifNull(total_rows, 0) AS rows, ifNull(total_bytes, 0) AS size,
       metadata_modification_time AS modified, comment
FROM system.tables
WHERE database NOT IN ('INFORMATION_SCHEMA', 'information_schema', 'system')
  AND engine `+op+` ('View', 'MaterializedView', 'LiveView', 'WindowView')
  AND (? = '' OR database = ?)
  AND (? = '' OR name = ?)
ORDER BY database, name`, []any{database, database, table, table})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name, db := fmt.Sprint(r["name"]), fmt.Sprint(r["database"])
		r["ref"] = plugin.ResourceRef{Kind: refKind, Namespace: db, Name: name, UID: db + "." + name}
	}
	return pageRows(rc, rows)
}

func listDictionaries(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	database, table, err := optionalTableFilter(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT name, database, status, type, origin, bytes_allocated, element_count
FROM system.dictionaries
WHERE database NOT IN ('INFORMATION_SCHEMA', 'information_schema', 'system')
  AND (? = '' OR database = ?)
  AND (? = '' OR name = ?)
ORDER BY database, name`, []any{database, database, table, table})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		name, db := fmt.Sprint(r["name"]), fmt.Sprint(r["database"])
		r["ref"] = plugin.ResourceRef{Kind: "dictionary", Namespace: db, Name: name, UID: db + "." + name}
	}
	return pageRows(rc, rows)
}

func dictionaryOverview(rc *plugin.RequestContext) (any, error) {
	return tableScopedOverview(rc, "dictionary", "name", listDictionaries)
}

func listMutations(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	database, table, err := optionalTableFilter(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT mutation_id, database, table, command, create_time, is_done, latest_fail_reason
FROM system.mutations
WHERE database NOT IN ('INFORMATION_SCHEMA', 'information_schema', 'system')
  AND (? = '' OR database = ?)
  AND (? = '' OR table = ?)
ORDER BY create_time DESC, database, table`, []any{database, database, table, table})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		id := mutationUID(r)
		r["ref"] = plugin.ResourceRef{Kind: "mutation", Name: fmt.Sprint(r["mutation_id"]), UID: id}
	}
	return pageRows(rc, rows)
}

func mutationOverview(rc *plugin.RequestContext) (any, error) {
	return overviewByUID(rc, "id", listMutations)
}

func listMerges(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT concat(database, '.', table, ':', result_part_name) AS id,
       database, table, elapsed, progress, num_parts
FROM system.merges
ORDER BY elapsed DESC, database, table`, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		id := fmt.Sprint(r["id"])
		r["ref"] = plugin.ResourceRef{Kind: "merge", Name: id, UID: id}
	}
	return pageRows(rc, rows)
}

func mergeOverview(rc *plugin.RequestContext) (any, error) {
	return overviewByUID(rc, "id", listMerges)
}

func listProcesses(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT query_id, user, address, elapsed, read_rows, memory_usage, query
FROM system.processes
ORDER BY elapsed DESC`, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		id := fmt.Sprint(r["query_id"])
		r["ref"] = plugin.ResourceRef{Kind: "process", Name: id, UID: id}
	}
	return pageRows(rc, rows)
}

func processOverview(rc *plugin.RequestContext) (any, error) {
	return overviewByUID(rc, "id", listProcesses)
}

func listUsers(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT name AS user, auth_type, storage
FROM system.users
ORDER BY name`, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		user := fmt.Sprint(r["user"])
		r["ref"] = plugin.ResourceRef{Kind: "user", Name: user, UID: user}
	}
	return pageRows(rc, rows)
}

func userOverview(rc *plugin.RequestContext) (any, error) {
	user, err := sqldb.SafeIdentifier(rc.Param("user"))
	if err != nil {
		return nil, err
	}
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT name AS user, auth_type, storage
FROM system.users
WHERE name = ?`, []any{user})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

// columnNames returns a table's column names in order, for the data grid's
// free-text search across every column.
func columnNames(ctx context.Context, s *Session, database, table string) ([]string, error) {
	rows, err := queryRows(ctx, s, "SELECT name FROM system.columns WHERE database = ? AND table = ? ORDER BY position", []any{database, table})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprint(r["name"]))
	}
	return out, nil
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
	s, err := clickhouseSession(rc)
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
	filter := req.Search()
	var cols []string
	if filter != "" {
		cols, err = columnNames(rc.Ctx, s, database, table)
		if err != nil {
			return nil, err
		}
	}
	searchDialect := sqldb.Dialect{QuoteIdent: quoteIdent, Placeholder: sqldb.QuestionPlaceholder}
	searchClause, searchArgs := searchDialect.SearchClause("String", cols, filter, 1)
	where := ""
	if searchClause != "" {
		where = " WHERE " + searchClause
	}
	var total uint64
	if err := s.db.QueryRowContext(rc.Ctx, "SELECT count() FROM "+qualified(database, table)+where, searchArgs...).Scan(&total); err != nil {
		return nil, clickhouseErr(err)
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
	dataArgs := append(append([]any{}, searchArgs...), limit, offset)
	rows, err := queryRows(rc.Ctx, s, fmt.Sprintf("SELECT * FROM %s%s%s LIMIT ? OFFSET ?", qualified(database, table), where, orderBy), dataArgs)
	if err != nil {
		return nil, err
	}
	redactRows(rows, s.opts.RedactPatterns)
	next := ""
	if uint64(offset+len(rows)) < total {
		next = strconv.Itoa(offset + len(rows))
	}
	totalInt := int(total)
	return plugin.Page[row]{Items: rows, NextCursor: next, Total: &totalInt}, nil
}

func tableColumnsRoute(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT name, type, default_kind, default_expression, position, comment
FROM system.columns
WHERE database = ? AND table = ?
ORDER BY position`, []any{database, table})
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
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT name, expr AS expression, type, granularity
FROM system.data_skipping_indices
WHERE database = ? AND table = ?
ORDER BY name`, []any{database, table})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		name := fmt.Sprint(rows[i]["name"])
		rows[i]["ref"] = plugin.ResourceRef{Kind: "index", Scope: database, Namespace: table, Name: name, UID: database + "." + table + "." + name}
	}
	return pageRows(rc, rows)
}

func tableConstraints(rc *plugin.RequestContext) (any, error) {
	return pageRows(rc, []row{})
}

func tableDefinition(rc *plugin.RequestContext) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT database, name, engine, create_table_query AS definition
FROM system.tables
WHERE database = ? AND name = ?`, []any{database, table})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, plugin.ErrNotFound
	}
	return rows[0], nil
}

func completionRoute(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	items := []sqldb.CompletionItem{
		{Label: "SELECT", Type: "keyword"},
		{Label: "FROM", Type: "keyword"},
		{Label: "WHERE", Type: "keyword"},
		{Label: "GROUP BY", Type: "keyword"},
		{Label: "ORDER BY", Type: "keyword"},
		{Label: "LIMIT", Type: "keyword"},
		{Label: "INSERT INTO", Type: "keyword"},
		{Label: "ALTER TABLE", Type: "keyword"},
		{Label: "OPTIMIZE TABLE", Type: "keyword"},
		{Label: "SYSTEM", Type: "keyword"},
	}
	rows, err := queryRows(rc.Ctx, s, `
SELECT c.database, c.table, t.engine, c.name AS column
FROM system.columns c
JOIN system.tables t ON t.database = c.database AND t.name = c.table
WHERE c.database NOT IN ('INFORMATION_SCHEMA', 'information_schema', 'system')
ORDER BY c.database, c.table, c.position
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
		database := fmt.Sprint(r["database"])
		relation := fmt.Sprint(r["table"])
		kind := "table"
		if strings.Contains(strings.ToLower(fmt.Sprint(r["engine"])), "view") {
			kind = "view"
		}
		add(sqldb.CompletionItem{Label: database, Type: "namespace", Detail: "database"})
		add(sqldb.CompletionItem{Label: relation, Type: kind, Detail: database, Apply: quoteIdent(database) + "." + quoteIdent(relation)})
		column := fmt.Sprint(r["column"])
		if column != "" && column != "<nil>" {
			add(sqldb.CompletionItem{Label: column, Type: "property", Detail: database + "." + relation})
		}
	}
	return items, nil
}

func createDatabase(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	var req struct {
		Name        string `json:"name" validate:"required"`
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
	if _, err := s.db.ExecContext(rc.Ctx, prefix+quoteIdent(name)); err != nil {
		return nil, clickhouseErr(err)
	}
	return actionResult{OK: true}, nil
}

func createTable(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
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
		Engine      string `json:"engine" validate:"required"`
		OrderBy     string `json:"order_by" validate:"required"`
		IfNotExists bool   `json:"if_not_exists"`
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
		engine = "MergeTree"
	}
	if !sqldb.SafeType(engine) {
		return nil, fmt.Errorf("%w: unsafe table engine", plugin.ErrInvalidInput)
	}
	orderBy := strings.TrimSpace(req.OrderBy)
	if orderBy == "" {
		orderBy = "tuple()"
	}
	if !sqldb.SafeDefault(orderBy) {
		return nil, fmt.Errorf("%w: unsafe ORDER BY expression", plugin.ErrInvalidInput)
	}
	prefix := "CREATE TABLE "
	if req.IfNotExists {
		prefix += "IF NOT EXISTS "
	}
	sqlText := prefix + qualified(database, table) + " (" + strings.Join(columns, ", ") + ") ENGINE = " + engine + " ORDER BY " + orderBy
	if _, err := s.db.ExecContext(rc.Ctx, sqlText); err != nil {
		return nil, clickhouseErr(err)
	}
	return actionResult{OK: true}, nil
}

func addColumn(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
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
		return nil, clickhouseErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropColumn(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
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
		return nil, clickhouseErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropIndex(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
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
	// ClickHouse data-skipping indexes are dropped via ALTER TABLE ... DROP INDEX.
	if _, err := s.db.ExecContext(rc.Ctx, "ALTER TABLE "+qualified(database, table)+" DROP INDEX "+quoteIdent(name)); err != nil {
		return nil, clickhouseErr(err)
	}
	return actionResult{OK: true}, nil
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
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(rc.Ctx, sqlText); err != nil {
		return nil, clickhouseErr(err)
	}
	return actionResult{OK: true}, nil
}

func cancelQuery(rc *plugin.RequestContext) (any, error) {
	s, err := clickhouseSession(rc)
	if err != nil {
		return nil, err
	}
	return actionResult{OK: s.cancelAll()}, nil
}

func queryStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := clickhouseSession(rc)
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
				payload["confirmMessage"] = "This ClickHouse statement can change data, schema, privileges, or server state. Review it before running."
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
			if !isReadOnlyStatement(st) {
				return sqldb.QueryResult{}, fmt.Errorf("%w: read-only mode blocks write statements", plugin.ErrForbidden)
			}
		}
	}
	if s.opts.RequireConfirm && !req.Confirm {
		for _, st := range statements {
			if isDestructiveStatement(st) {
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
			return sqldb.StatementResult{}, clickhouseErr(err)
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
		return sqldb.StatementResult{}, clickhouseErr(err)
	}
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return sqldb.StatementResult{}, clickhouseErr(err)
	}
	out := sqldb.StatementResult{Statement: statement, Columns: columns}
	for rows.Next() {
		values, err := scanValues(rows, columns)
		if err != nil {
			_ = rows.Close()
			return sqldb.StatementResult{}, clickhouseErr(err)
		}
		out.Rows = append(out.Rows, values)
		if len(out.Rows) >= s.opts.RowLimit {
			break
		}
	}
	if err := rows.Close(); err != nil {
		return sqldb.StatementResult{}, clickhouseErr(err)
	}
	if err := rows.Err(); err != nil {
		return sqldb.StatementResult{}, clickhouseErr(err)
	}
	out.RowCount = int64(len(out.Rows))
	out.CommandTag = sqldb.FirstKeyword(statement)
	out.Rows = sqldb.RedactRows(out.Columns, out.Rows, s.opts.RedactPatterns)
	out.ElapsedMS = time.Since(start).Milliseconds()
	return out, nil
}

func statementReturnsRows(statement string) bool {
	switch sqldb.FirstKeyword(statement) {
	case "SELECT", "SHOW", "EXPLAIN", "WITH", "DESCRIBE", "DESC", "CHECK":
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
		if isDestructiveStatement(st) {
			return true
		}
	}
	return false
}

func isReadOnlyStatement(statement string) bool {
	switch sqldb.FirstKeyword(statement) {
	case "SELECT", "SHOW", "EXPLAIN", "WITH", "DESCRIBE", "DESC", "CHECK":
		return true
	default:
		return false
	}
}

func isDestructiveStatement(statement string) bool {
	switch sqldb.FirstKeyword(statement) {
	case "INSERT", "ALTER", "DELETE", "DROP", "TRUNCATE", "CREATE", "GRANT", "REVOKE", "OPTIMIZE", "SYSTEM", "KILL", "ATTACH", "DETACH", "RENAME", "EXCHANGE", "BACKUP", "RESTORE", "UNDROP":
		return true
	default:
		return false
	}
}

func queryRows(ctx context.Context, s *Session, sqlText string, args []any) ([]row, error) {
	ctx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, clickhouseErr(err)
	}
	defer func() { _ = rows.Close() }()
	columns, err := rows.Columns()
	if err != nil {
		return nil, clickhouseErr(err)
	}
	out := []row{}
	for rows.Next() {
		values, err := scanValues(rows, columns)
		if err != nil {
			return nil, clickhouseErr(err)
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
		return nil, clickhouseErr(err)
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

func tableScopedOverview(rc *plugin.RequestContext, kind string, key string, load func(*plugin.RequestContext) (any, error)) (any, error) {
	database, table, err := tableIdent(rc)
	if err != nil {
		return nil, err
	}
	res, err := load(rc)
	if err != nil {
		return nil, err
	}
	page, ok := res.(plugin.Page[row])
	if !ok {
		return nil, fmt.Errorf("%w: overview source returned invalid page", plugin.ErrUnavailable)
	}
	for _, item := range page.Items {
		if fmt.Sprint(item["database"]) == database && fmt.Sprint(item[key]) == table {
			return item, nil
		}
		if ref, ok := item["ref"].(plugin.ResourceRef); ok && ref.Kind == kind && ref.UID == database+"."+table {
			return item, nil
		}
	}
	return nil, plugin.ErrNotFound
}

func overviewByUID(rc *plugin.RequestContext, param string, load func(*plugin.RequestContext) (any, error)) (any, error) {
	id := strings.TrimSpace(rc.Param(param))
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", plugin.ErrInvalidInput)
	}
	res, err := load(rc)
	if err != nil {
		return nil, err
	}
	page, ok := res.(plugin.Page[row])
	if !ok {
		return nil, fmt.Errorf("%w: overview source returned invalid page", plugin.ErrUnavailable)
	}
	for _, item := range page.Items {
		if ref, ok := item["ref"].(plugin.ResourceRef); ok && ref.UID == id {
			return item, nil
		}
	}
	return nil, plugin.ErrNotFound
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

func clickhouseErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return plugin.ErrNotFound
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

func optionalTableFilter(rc *plugin.RequestContext) (string, string, error) {
	database, err := sqldb.OptionalIdentifier(firstNonEmpty(rc.Query().Get("p.database"), rc.Param("database")))
	if err != nil {
		return "", "", err
	}
	table, err := sqldb.OptionalIdentifier(firstNonEmpty(rc.Query().Get("p.table"), rc.Param("table")))
	if err != nil {
		return "", "", err
	}
	return database, table, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func qualified(database, name string) string {
	return quoteIdent(database) + "." + quoteIdent(name)
}

func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func mutationUID(r row) string {
	return fmt.Sprintf("%s.%s:%s", r["database"], r["table"], r["mutation_id"])
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
	if spec.Nullable && !strings.HasPrefix(strings.ToLower(dataType), "nullable(") {
		dataType = "Nullable(" + dataType + ")"
	}
	parts := []string{quoteIdent(name), dataType}
	if strings.TrimSpace(spec.Default) != "" {
		if !sqldb.SafeDefault(spec.Default) {
			return "", fmt.Errorf("%w: unsafe default expression", plugin.ErrInvalidInput)
		}
		parts = append(parts, "DEFAULT "+strings.TrimSpace(spec.Default))
	}
	return strings.Join(parts, " "), nil
}

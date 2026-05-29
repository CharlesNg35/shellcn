package mongodb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

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
		{ID: "mongodb.databases.tree", Method: plugin.MethodGet, Path: "/tree/databases", Permission: "mongodb.databases.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.databases.tree", Handle: treeDatabases},
		{ID: "mongodb.databases.list", Method: plugin.MethodGet, Path: "/databases", Permission: "mongodb.databases.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.databases.list", Handle: listDatabases},
		{ID: "mongodb.database.create", Method: plugin.MethodPost, Path: "/databases", Permission: "mongodb.databases.write", Risk: plugin.RiskWrite, AuditEvent: "mongodb.database.create", Input: databaseCreateSchema(), Handle: createDatabase},
		{ID: "mongodb.database.overview", Method: plugin.MethodGet, Path: "/databases/{database}/overview", Permission: "mongodb.databases.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.database.overview", Handle: databaseOverview},
		{ID: "mongodb.collections.tree", Method: plugin.MethodGet, Path: "/tree/collections", Permission: "mongodb.collections.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.collections.tree", Handle: treeCollections},
		{ID: "mongodb.collections.list", Method: plugin.MethodGet, Path: "/collections", Permission: "mongodb.collections.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.collections.list", Handle: listCollections},
		{ID: "mongodb.collection.stats", Method: plugin.MethodGet, Path: "/collections/{database}/{collection}/stats", Permission: "mongodb.collections.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.collection.stats", Handle: collectionStats},
		{ID: "mongodb.indexes.list", Method: plugin.MethodGet, Path: "/collections/{database}/{collection}/indexes", Permission: "mongodb.indexes.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.indexes.list", Handle: listIndexes},
		{ID: "mongodb.index.create", Method: plugin.MethodPost, Path: "/collections/{database}/{collection}/indexes", Permission: "mongodb.indexes.write", Risk: plugin.RiskWrite, AuditEvent: "mongodb.index.create", Input: indexCreateSchema(), Handle: createIndex},
		{ID: "mongodb.index.drop", Method: plugin.MethodDelete, Path: "/collections/{database}/{collection}/indexes/{name}", Permission: "mongodb.indexes.delete", Risk: plugin.RiskDestructive, AuditEvent: "mongodb.index.drop", Handle: dropIndex},
		{ID: "mongodb.documents.list", Method: plugin.MethodGet, Path: "/collections/{database}/{collection}/documents", Permission: "mongodb.documents.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.documents.list", Handle: listDocuments},
		{ID: "mongodb.document.read", Method: plugin.MethodGet, Path: "/documents/{id}", Permission: "mongodb.documents.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.document.read", Handle: readDocument},
		{ID: "mongodb.collection.create", Method: plugin.MethodPost, Path: "/databases/{database}/collections", Permission: "mongodb.collections.write", Risk: plugin.RiskWrite, AuditEvent: "mongodb.collection.create", Input: collectionCreateSchema(), Handle: createCollection},
		{ID: "mongodb.collection.drop", Method: plugin.MethodDelete, Path: "/collections/{database}/{collection}", Permission: "mongodb.collections.delete", Risk: plugin.RiskDestructive, AuditEvent: "mongodb.collection.drop", Handle: dropCollection},
		{ID: "mongodb.document.create", Method: plugin.MethodPost, Path: "/collections/{database}/{collection}/documents", Permission: "mongodb.documents.write", Risk: plugin.RiskWrite, AuditEvent: "mongodb.document.create", Input: documentCreateSchema(), Handle: createDocument},
		{ID: "mongodb.document.update", Method: plugin.MethodPut, Path: "/documents/{id}", Permission: "mongodb.documents.write", Risk: plugin.RiskWrite, AuditEvent: "mongodb.document.update", Handle: updateDocument},
		{ID: "mongodb.document.delete", Method: plugin.MethodDelete, Path: "/documents/{id}", Permission: "mongodb.documents.delete", Risk: plugin.RiskDestructive, AuditEvent: "mongodb.document.delete", Handle: deleteDocument},
		{ID: "mongodb.command", Method: plugin.MethodWS, Path: "/command", Permission: "mongodb.command.execute", Risk: plugin.RiskPrivileged, AuditEvent: "mongodb.command", Stream: commandStream},
		{ID: "mongodb.completion", Method: plugin.MethodGet, Path: "/completion", Permission: "mongodb.databases.read", Risk: plugin.RiskSafe, AuditEvent: "mongodb.completion", Handle: completionRoute},
	}
}

func mongoSession(rc *plugin.RequestContext) (*Session, error) {
	return unwrap(rc.Session)
}

func databaseCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Database", Fields: []plugin.Field{
		{Key: "name", Label: "Database name", Type: plugin.FieldText, Required: true},
		{Key: "collection", Label: "First collection", Type: plugin.FieldText, Required: true, Help: "A database is created with its first collection."},
	}}}}
}

func collectionCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Collection", Fields: []plugin.Field{
		{Key: "name", Label: "Collection name", Type: plugin.FieldText, Required: true},
		{Key: "capped", Label: "Capped", Type: plugin.FieldToggle, Default: false},
		{Key: "size", Label: "Max size bytes", Type: plugin.FieldNumber, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}}},
	}}}}
}

func indexCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Index", Fields: []plugin.Field{
		{Key: "keys", Label: "Keys", Type: plugin.FieldJSON, Required: true, Help: `Field-to-direction map, e.g. {"email":1,"createdAt":-1}.`},
		{Key: "name", Label: "Index name", Type: plugin.FieldText, Help: "Optional; derived from the keys when blank."},
		{Key: "unique", Label: "Unique", Type: plugin.FieldToggle},
		{Key: "sparse", Label: "Sparse", Type: plugin.FieldToggle},
	}}}}
}

func documentCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Document", Fields: []plugin.Field{
		{Key: "document", Label: "Document", Type: plugin.FieldJSON, Required: true, Help: "MongoDB Extended JSON document."},
	}}}}
}

func treeDatabases(rc *plugin.RequestContext) (any, error) {
	res, err := listDatabases(rc)
	if err != nil {
		return nil, err
	}
	page := res.(plugin.Page[row])
	nodes := make([]plugin.TreeNode, 0, len(page.Items))
	for _, item := range page.Items {
		name := fmt.Sprint(item["name"])
		ref := plugin.ResourceRef{Kind: "database", Name: name, UID: name}
		nodes = append(nodes, plugin.TreeNode{
			Key:            "database:" + name,
			Label:          name,
			Icon:           icon("database"),
			Ref:            &ref,
			ChildrenSource: &plugin.DataSource{RouteID: "mongodb.collections.tree", Params: map[string]string{"database": name}},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func treeCollections(rc *plugin.RequestContext) (any, error) {
	res, err := listCollections(rc)
	if err != nil {
		return nil, err
	}
	page := res.(plugin.Page[row])
	nodes := make([]plugin.TreeNode, 0, len(page.Items))
	for _, item := range page.Items {
		name, database := fmt.Sprint(item["name"]), fmt.Sprint(item["database"])
		ref := plugin.ResourceRef{Kind: "collection", Namespace: database, Name: name, UID: database + "." + name}
		nodes = append(nodes, plugin.TreeNode{Key: "collection:" + ref.UID, Label: name, Icon: icon("folder"), Ref: &ref, Leaf: true})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func listDatabases(rc *plugin.RequestContext) (any, error) {
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	result, err := s.client.ListDatabases(ctx, bson.D{})
	if err != nil {
		return nil, mongoErr(err)
	}
	rows := make([]row, 0, len(result.Databases))
	for _, db := range result.Databases {
		if isInternalDatabase(db.Name) {
			continue
		}
		rows = append(rows, row{
			"name":  db.Name,
			"size":  db.SizeOnDisk,
			"empty": db.Empty,
			"ref":   plugin.ResourceRef{Kind: "database", Name: db.Name, UID: db.Name},
		})
	}
	return pageRows(rc, rows)
}

func databaseOverview(rc *plugin.RequestContext) (any, error) {
	database, err := safeName(rc.Param("database"), "database")
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	var stats bson.M
	if err := s.client.Database(database).RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&stats); err != nil {
		return nil, mongoErr(err)
	}
	return bsonDoc(stats)
}

func listCollections(rc *plugin.RequestContext) (any, error) {
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	database := strings.TrimSpace(rc.Query().Get("p.database"))
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	databases := []string{database}
	if database == "" {
		list, err := s.client.ListDatabaseNames(ctx, bson.D{})
		if err != nil {
			return nil, mongoErr(err)
		}
		databases = list
	}
	rows := []row{}
	for _, dbName := range databases {
		if dbName == "" || isInternalDatabase(dbName) {
			continue
		}
		if _, err := safeName(dbName, "database"); err != nil {
			return nil, err
		}
		db := s.client.Database(dbName)
		cur, err := db.ListCollections(ctx, bson.D{})
		if err != nil {
			return nil, mongoErr(err)
		}
		var collections []bson.M
		if err := cur.All(ctx, &collections); err != nil {
			return nil, mongoErr(err)
		}
		for _, coll := range collections {
			name := fmt.Sprint(coll["name"])
			if name == "" || strings.HasPrefix(name, "system.") {
				continue
			}
			count, _ := db.Collection(name).EstimatedDocumentCount(ctx)
			var stats bson.M
			_ = db.RunCommand(ctx, bson.D{{Key: "collStats", Value: name}}).Decode(&stats)
			rows = append(rows, row{
				"name":     name,
				"database": dbName,
				"type":     fmt.Sprint(coll["type"]),
				"count":    count,
				"size":     numberValue(stats["size"]),
				"ref":      plugin.ResourceRef{Kind: "collection", Namespace: dbName, Name: name, UID: dbName + "." + name},
			})
		}
	}
	return pageRows(rc, rows)
}

func collectionStats(rc *plugin.RequestContext) (any, error) {
	database, collection, err := collectionIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	var stats bson.M
	if err := s.client.Database(database).RunCommand(ctx, bson.D{{Key: "collStats", Value: collection}}).Decode(&stats); err != nil {
		return nil, mongoErr(err)
	}
	return bsonDoc(stats)
}

func listIndexes(rc *plugin.RequestContext) (any, error) {
	database, collection, err := collectionIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	cur, err := s.client.Database(database).Collection(collection).Indexes().List(ctx)
	if err != nil {
		return nil, mongoErr(err)
	}
	var indexes []bson.M
	if err := cur.All(ctx, &indexes); err != nil {
		return nil, mongoErr(err)
	}
	rows := make([]row, 0, len(indexes))
	for _, idx := range indexes {
		name := fmt.Sprint(idx["name"])
		rows = append(rows, row{
			"name":   name,
			"keys":   compactJSON(idx["key"]),
			"unique": boolField(idx["unique"]),
			"sparse": boolField(idx["sparse"]),
			"ref":    plugin.ResourceRef{Kind: "index", Scope: database, Namespace: collection, Name: name, UID: database + "." + collection + "." + name},
		})
	}
	return pageRows(rc, rows)
}

func createIndex(rc *plugin.RequestContext) (any, error) {
	database, collection, err := collectionIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	var req struct {
		Keys   any    `json:"keys" validate:"required"`
		Name   string `json:"name"`
		Unique bool   `json:"unique"`
		Sparse bool   `json:"sparse"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	keys, err := indexKeys(req.Keys)
	if err != nil {
		return nil, err
	}
	opts := options.Index().SetUnique(req.Unique).SetSparse(req.Sparse)
	if name := strings.TrimSpace(req.Name); name != "" {
		if _, err := safeName(name, "index"); err != nil {
			return nil, err
		}
		opts.SetName(name)
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	if _, err := s.client.Database(database).Collection(collection).Indexes().CreateOne(ctx, mongo.IndexModel{Keys: keys, Options: opts}); err != nil {
		return nil, mongoErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropIndex(rc *plugin.RequestContext) (any, error) {
	database, collection, err := collectionIdent(rc)
	if err != nil {
		return nil, err
	}
	name, err := safeName(rc.Param("name"), "index")
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	if err := s.client.Database(database).Collection(collection).Indexes().DropOne(ctx, name); err != nil {
		return nil, mongoErr(err)
	}
	return actionResult{OK: true}, nil
}

// indexKeys parses a field-to-direction map into an ordered key document, so a
// compound index keeps the field order the user wrote.
func indexKeys(value any) (bson.D, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: index keys must be a JSON object", plugin.ErrInvalidInput)
	}
	var keys bson.D
	if err := bson.UnmarshalExtJSON(raw, false, &keys); err != nil || len(keys) == 0 {
		return nil, fmt.Errorf("%w: index keys must be a non-empty field-to-direction map", plugin.ErrInvalidInput)
	}
	return keys, nil
}

func listDocuments(rc *plugin.RequestContext) (any, error) {
	database, collection, err := collectionIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	filter, err := filterDocument(req.Search())
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit > s.opts.DocumentLimit {
		limit = s.opts.DocumentLimit
	}
	offset, err := offsetCursor(req.Cursor)
	if err != nil {
		return nil, err
	}
	findOpts := options.Find().SetLimit(int64(limit)).SetSkip(int64(offset))
	if len(req.Sort) > 0 {
		dir := int32(1)
		if req.Sort[0].Desc {
			dir = -1
		}
		findOpts.SetSort(bson.D{{Key: req.Sort[0].Field, Value: dir}})
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	coll := s.client.Database(database).Collection(collection)
	total64, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, mongoErr(err)
	}
	cur, err := coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, mongoErr(err)
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, mongoErr(err)
	}
	rows := make([]row, 0, len(docs))
	for _, doc := range docs {
		item, err := documentRow(database, collection, doc)
		if err != nil {
			return nil, err
		}
		rows = append(rows, item)
	}
	total := int(total64)
	next := ""
	if offset+len(rows) < total {
		next = strconv.Itoa(offset + len(rows))
	}
	return plugin.Page[row]{Items: rows, NextCursor: next, Total: &total}, nil
}

func readDocument(rc *plugin.RequestContext) (any, error) {
	database, collection, filter, err := documentFilter(rc.Param("id"))
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	var doc bson.M
	if err := s.client.Database(database).Collection(collection).FindOne(ctx, filter).Decode(&doc); err != nil {
		return nil, mongoErr(err)
	}
	return bsonDoc(doc)
}

func createDatabase(rc *plugin.RequestContext) (any, error) {
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	var req struct {
		Name       string `json:"name" validate:"required"`
		Collection string `json:"collection" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	database, err := safeName(req.Name, "database")
	if err != nil {
		return nil, err
	}
	collection, err := safeName(req.Collection, "collection")
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	// MongoDB has no standalone "create database"; the database springs into
	// existence with its first collection.
	if err := s.client.Database(database).CreateCollection(ctx, collection); err != nil {
		return nil, mongoErr(err)
	}
	return actionResult{OK: true}, nil
}

func createCollection(rc *plugin.RequestContext) (any, error) {
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	database, err := safeName(rc.Param("database"), "database")
	if err != nil {
		return nil, err
	}
	var req struct {
		Name   string `json:"name" validate:"required"`
		Capped bool   `json:"capped"`
		Size   int64  `json:"size"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	collection, err := safeName(req.Name, "collection")
	if err != nil {
		return nil, err
	}
	opts := options.CreateCollection()
	if req.Capped {
		opts.SetCapped(true)
		if req.Size > 0 {
			opts.SetSizeInBytes(req.Size)
		}
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	if err := s.client.Database(database).CreateCollection(ctx, collection, opts); err != nil {
		return nil, mongoErr(err)
	}
	return actionResult{OK: true}, nil
}

func dropCollection(rc *plugin.RequestContext) (any, error) {
	database, collection, err := collectionIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	if err := s.client.Database(database).Collection(collection).Drop(ctx); err != nil {
		return nil, mongoErr(err)
	}
	return actionResult{OK: true}, nil
}

func createDocument(rc *plugin.RequestContext) (any, error) {
	database, collection, err := collectionIdent(rc)
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	var req struct {
		Document any `json:"document" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	doc, err := requestDocument(req.Document)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	res, err := s.client.Database(database).Collection(collection).InsertOne(ctx, doc)
	if err != nil {
		return nil, mongoErr(err)
	}
	return map[string]any{"ok": true, "id": fmt.Sprint(res.InsertedID)}, nil
}

func updateDocument(rc *plugin.RequestContext) (any, error) {
	database, collection, filter, err := documentFilter(rc.Param("id"))
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	var req struct {
		Content string `json:"content" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	doc, err := parseExtJSON(req.Content)
	if err != nil {
		return nil, err
	}
	if _, ok := doc["_id"]; !ok {
		doc["_id"] = filter[0].Value
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	res, err := s.client.Database(database).Collection(collection).ReplaceOne(ctx, filter, doc)
	if err != nil {
		return nil, mongoErr(err)
	}
	if res.MatchedCount == 0 {
		return nil, plugin.ErrNotFound
	}
	return actionResult{OK: true}, nil
}

func deleteDocument(rc *plugin.RequestContext) (any, error) {
	database, collection, filter, err := documentFilter(rc.Param("id"))
	if err != nil {
		return nil, err
	}
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	res, err := s.client.Database(database).Collection(collection).DeleteOne(ctx, filter)
	if err != nil {
		return nil, mongoErr(err)
	}
	if res.DeletedCount == 0 {
		return nil, plugin.ErrNotFound
	}
	return actionResult{OK: true}, nil
}

func commandStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := mongoSession(rc)
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
			if err := enc.Encode(map[string]any{"error": "Invalid MongoDB command request."}); err != nil {
				return err
			}
			continue
		}
		database := stringDefault(rc.Param("database"), s.opts.Database)
		result, err := executeCommandRequest(client.Context(), s, database, req)
		rc.Audit(commandAuditResult(err), commandAuditParams(req.Query, result, err), err)
		if err != nil {
			payload := map[string]any{"error": err.Error()}
			var confirmErr confirmationError
			if errors.As(err, &confirmErr) {
				payload["requiresConfirmation"] = true
				payload["confirmMessage"] = "This MongoDB command can change data, schema, or server state. Review it before running."
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

func completionRoute(*plugin.RequestContext) (any, error) {
	commands := []string{
		`{"find": "collection", "filter": {}, "limit": 50}`,
		`{"aggregate": "collection", "pipeline": []}`,
		`{"count": "collection", "query": {}}`,
		`{"distinct": "collection", "key": "field", "query": {}}`,
		`{"listCollections": 1}`,
		`{"dbStats": 1}`,
		`{"serverStatus": 1}`,
		`{"buildInfo": 1}`,
	}
	items := make([]sqldb.CompletionItem, 0, len(commands))
	for _, command := range commands {
		items = append(items, sqldb.CompletionItem{Label: command, Type: "keyword", Apply: command})
	}
	return items, nil
}

func executeCommandRequest(parent context.Context, s *Session, database string, req sqldb.QueryRequest) (sqldb.QueryResult, error) {
	command, err := parseExtJSON(req.Query)
	if err != nil {
		return sqldb.QueryResult{}, err
	}
	name := commandName(command)
	if name == "" {
		return sqldb.QueryResult{}, fmt.Errorf("%w: command document is empty", plugin.ErrInvalidInput)
	}
	if s.opts.ReadOnly && !isReadOnlyCommand(name, command) {
		return sqldb.QueryResult{}, fmt.Errorf("%w: read-only mode blocks write commands", plugin.ErrForbidden)
	}
	if s.opts.RequireConfirm && !req.Confirm && !isReadOnlyCommand(name, command) {
		return sqldb.QueryResult{}, confirmationError{message: "command requires confirmation"}
	}
	ctx, cancel := commandContext(parent, s)
	defer cancel()
	start := time.Now()
	var out bson.M
	if err := s.client.Database(database).RunCommand(ctx, orderedCommand(command)).Decode(&out); err != nil {
		return sqldb.QueryResult{}, mongoErr(err)
	}
	columns, rows := commandRows(out)
	return sqldb.QueryResult{
		Columns:    columns,
		Rows:       rows,
		RowCount:   int64(len(rows)),
		ElapsedMS:  time.Since(start).Milliseconds(),
		Statement:  req.Query,
		CommandTag: name,
	}, nil
}

func commandRows(doc bson.M) ([]string, [][]any) {
	plain := bsonMap(doc)
	if cursor, ok := plain["cursor"].(map[string]any); ok {
		if batch, ok := cursor["firstBatch"].([]any); ok {
			return docsRows(batch)
		}
	}
	if cursor, ok := doc["cursor"].(bson.M); ok {
		if batch, ok := cursor["firstBatch"].(bson.A); ok {
			return docsRows(batch)
		}
	}
	return docRows(doc)
}

func docsRows(values []any) ([]string, [][]any) {
	columns := []string{}
	seen := map[string]bool{}
	rows := make([][]any, 0, len(values))
	mapped := make([]map[string]any, 0, len(values))
	for _, value := range values {
		plain, _ := value.(map[string]any)
		if doc, ok := value.(bson.M); ok {
			plain = bsonMap(doc)
		}
		if plain == nil {
			plain = map[string]any{"value": value}
		}
		mapped = append(mapped, plain)
		for key := range plain {
			if !seen[key] {
				seen[key] = true
				columns = append(columns, key)
			}
		}
	}
	sort.Strings(columns)
	for _, doc := range mapped {
		row := make([]any, 0, len(columns))
		for _, key := range columns {
			row = append(row, displayValue(key, doc[key]))
		}
		rows = append(rows, row)
	}
	return columns, rows
}

func docRows(doc bson.M) ([]string, [][]any) {
	plain := bsonMap(doc)
	keys := make([]string, 0, len(plain))
	for key := range plain {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]any, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []any{key, displayValue(key, plain[key])})
	}
	return []string{"key", "value"}, rows
}

func pageRows(rc *plugin.RequestContext, rows []row) (plugin.Page[row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[row]{}, err
	}
	rows = filterRows(rows, req.Search())
	sortRows(rows, req.Sort)
	total := len(rows)
	start, err := offsetCursor(req.Cursor)
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

func filterRows(rows []row, q string) []row {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		for _, value := range r {
			if strings.Contains(strings.ToLower(fmt.Sprint(value)), q) {
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

func documentRow(database, collection string, doc bson.M) (row, error) {
	id, ok := doc["_id"]
	if !ok {
		return nil, fmt.Errorf("%w: document is missing _id", plugin.ErrUnavailable)
	}
	encoded, err := encodeDocumentID(database, collection, id)
	if err != nil {
		return nil, err
	}
	out := row{}
	for key, value := range bsonMap(doc) {
		out[key] = displayValue(key, value)
	}
	out["ref"] = plugin.ResourceRef{Kind: "document", Name: idLabel(id), UID: encoded}
	return out, nil
}

func bsonDoc(doc any) (map[string]any, error) {
	raw, err := bson.MarshalExtJSON(doc, false, false)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func bsonMap(doc bson.M) map[string]any {
	out, err := bsonDoc(doc)
	if err != nil {
		return map[string]any{}
	}
	return out
}

func compactJSON(value any) string {
	raw, err := json.Marshal(value)
	if err == nil {
		return string(raw)
	}
	raw, err = bson.MarshalExtJSON(value, false, false)
	if err == nil {
		return string(raw)
	}
	return fmt.Sprint(value)
}

func displayValue(key string, value any) any {
	if display, ok := idDisplayValue(key, value); ok {
		return display
	}
	switch v := value.(type) {
	case map[string]any, []any:
		return compactJSON(v)
	default:
		return v
	}
}

func idDisplayValue(key string, value any) (string, bool) {
	if !sqldb.IDLikeColumn(key) {
		return "", false
	}
	switch v := value.(type) {
	case map[string]any:
		if oid, ok := stringMapValue(v, "$oid"); ok {
			return oid, true
		}
		rawBinary, ok := v["$binary"].(map[string]any)
		if !ok {
			return "", false
		}
		data, ok := stringMapValue(rawBinary, "base64")
		if !ok {
			return "", false
		}
		raw, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", false
		}
		return sqldb.FormatBinaryID(key, raw)
	case []any:
		raw, ok := byteSlice(v)
		if !ok {
			return "", false
		}
		return sqldb.FormatBinaryID(key, raw)
	default:
		return "", false
	}
}

func stringMapValue(values map[string]any, key string) (string, bool) {
	value, ok := values[key].(string)
	return value, ok && strings.TrimSpace(value) != ""
}

func byteSlice(values []any) ([]byte, bool) {
	out := make([]byte, 0, len(values))
	for _, value := range values {
		b, ok := byteValue(value)
		if !ok {
			return nil, false
		}
		out = append(out, b)
	}
	return out, true
}

func byteValue(value any) (byte, bool) {
	switch v := value.(type) {
	case int:
		if v >= 0 && v <= 255 {
			return byte(v), true
		}
	case int32:
		if v >= 0 && v <= 255 {
			return byte(v), true
		}
	case int64:
		if v >= 0 && v <= 255 {
			return byte(v), true
		}
	case float64:
		if v >= 0 && v <= 255 && v == float64(byte(v)) {
			return byte(v), true
		}
	}
	return 0, false
}

func requestDocument(value any) (bson.M, error) {
	if s, ok := value.(string); ok {
		return parseExtJSON(s)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: document must be JSON", plugin.ErrInvalidInput)
	}
	return parseExtJSON(string(raw))
}

func parseExtJSON(raw string) (bson.M, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: document is empty", plugin.ErrInvalidInput)
	}
	var doc bson.M
	if err := bson.UnmarshalExtJSON([]byte(raw), false, &doc); err != nil {
		return nil, fmt.Errorf("%w: invalid MongoDB Extended JSON: %v", plugin.ErrInvalidInput, err)
	}
	return doc, nil
}

func filterDocument(raw string) (bson.M, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return bson.M{}, nil
	}
	if strings.HasPrefix(raw, "{") {
		return parseExtJSON(raw)
	}
	if oid, err := bson.ObjectIDFromHex(raw); err == nil {
		return bson.M{"_id": oid}, nil
	}
	return bson.M{}, nil
}

type documentIdentity struct {
	Database   string `json:"database"`
	Collection string `json:"collection"`
	ID         any    `json:"id"`
}

func encodeDocumentID(database, collection string, id any) (string, error) {
	rawID, err := bson.MarshalExtJSON(bson.M{"_id": id}, false, false)
	if err != nil {
		return "", err
	}
	identity := documentIdentity{Database: database, Collection: collection, ID: json.RawMessage(rawID)}
	raw, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func documentFilter(encoded string) (string, string, bson.D, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: document id is invalid", plugin.ErrInvalidInput)
	}
	var identity struct {
		Database   string          `json:"database"`
		Collection string          `json:"collection"`
		ID         json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil {
		return "", "", nil, fmt.Errorf("%w: document id is invalid", plugin.ErrInvalidInput)
	}
	database, err := safeName(identity.Database, "database")
	if err != nil {
		return "", "", nil, err
	}
	collection, err := safeName(identity.Collection, "collection")
	if err != nil {
		return "", "", nil, err
	}
	idDoc, err := parseExtJSON(string(identity.ID))
	if err != nil {
		return "", "", nil, err
	}
	id, ok := idDoc["_id"]
	if !ok {
		return "", "", nil, fmt.Errorf("%w: document id is invalid", plugin.ErrInvalidInput)
	}
	return database, collection, bson.D{{Key: "_id", Value: id}}, nil
}

func idLabel(id any) string {
	switch v := id.(type) {
	case bson.ObjectID:
		return v.Hex()
	case bson.Binary:
		if display, ok := sqldb.FormatBinaryID("_id", v.Data); ok {
			return display
		}
	case bson.A:
		if raw, ok := byteSlice([]any(v)); ok {
			if display, ok := sqldb.FormatBinaryID("_id", raw); ok {
				return display
			}
		}
	case []any:
		if raw, ok := byteSlice(v); ok {
			if display, ok := sqldb.FormatBinaryID("_id", raw); ok {
				return display
			}
		}
	default:
		return fmt.Sprint(v)
	}
	return fmt.Sprint(id)
}

func orderedCommand(in bson.M) bson.D {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return bson.D{}
	}
	first := keys[0]
	for _, candidate := range []string{"find", "aggregate", "count", "distinct", "listCollections", "listIndexes", "dbStats", "collStats", "serverStatus", "buildInfo", "ping"} {
		if _, ok := in[candidate]; ok {
			first = candidate
			break
		}
	}
	out := bson.D{{Key: first, Value: in[first]}}
	sort.Strings(keys)
	for _, key := range keys {
		if key != first {
			out = append(out, bson.E{Key: key, Value: in[key]})
		}
	}
	return out
}

func commandName(command bson.M) string {
	for _, key := range []string{"find", "aggregate", "count", "distinct", "listCollections", "listIndexes", "dbStats", "collStats", "serverStatus", "buildInfo", "ping", "insert", "update", "delete", "drop", "dropDatabase", "create"} {
		if _, ok := command[key]; ok {
			return key
		}
	}
	for key := range command {
		return key
	}
	return ""
}

func isReadOnlyCommand(name string, command bson.M) bool {
	switch name {
	case "find", "count", "distinct", "listCollections", "listIndexes", "dbStats", "collStats", "serverStatus", "buildInfo", "ping":
		return true
	case "aggregate":
		return !pipelineWrites(command["pipeline"])
	default:
		return false
	}
}

func pipelineWrites(value any) bool {
	pipeline, ok := value.(bson.A)
	if !ok {
		return false
	}
	for _, stage := range pipeline {
		doc, ok := stage.(bson.M)
		if !ok {
			continue
		}
		if _, ok := doc["$out"]; ok {
			return true
		}
		if _, ok := doc["$merge"]; ok {
			return true
		}
	}
	return false
}

func commandAuditResult(err error) models.AuditResult {
	if err == nil {
		return models.AuditAllowed
	}
	var confirmErr confirmationError
	if errors.As(err, &confirmErr) || errors.Is(err, plugin.ErrForbidden) {
		return models.AuditDenied
	}
	return models.AuditError
}

func commandAuditParams(command string, result sqldb.QueryResult, err error) map[string]string {
	params := map[string]string{"command": commandNameForAudit(command), "hash": sqldb.QueryHash(command)}
	if result.ElapsedMS > 0 {
		params["elapsedMs"] = strconv.FormatInt(result.ElapsedMS, 10)
	}
	if err != nil {
		params["error"] = err.Error()
	}
	return params
}

func commandNameForAudit(raw string) string {
	doc, err := parseExtJSON(raw)
	if err != nil {
		return ""
	}
	return commandName(doc)
}

func collectionIdent(rc *plugin.RequestContext) (string, string, error) {
	database, err := safeName(rc.Param("database"), "database")
	if err != nil {
		return "", "", err
	}
	collection, err := safeName(rc.Param("collection"), "collection")
	if err != nil {
		return "", "", err
	}
	return database, collection, nil
}

func safeName(name, label string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: %s is required", plugin.ErrInvalidInput, label)
	}
	if strings.ContainsAny(name, "\x00/\\") || strings.HasPrefix(name, "$") {
		return "", fmt.Errorf("%w: %s is invalid", plugin.ErrInvalidInput, label)
	}
	return name, nil
}

func isInternalDatabase(name string) bool {
	return name == "admin" || name == "config" || name == "local"
}

func boolField(value any) bool {
	b, _ := value.(bool)
	return b
}

func numberValue(value any) int64 {
	switch n := value.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func offsetCursor(raw string) (int, error) {
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

func commandContext(parent context.Context, s *Session) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, s.opts.Timeout)
}

func mongoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return plugin.ErrNotFound
	}
	return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
}

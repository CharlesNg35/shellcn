package mongodb

import (
	"context"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestMongoDBPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_MONGODB_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_MONGODB_INTEGRATION=1 to run against MongoDB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := integrationConfig(ctx, t)
	cfg["read_only"] = false
	cfg["require_write_confirmation"] = true

	sess, err := connect(ctx, plugin.ConnectConfig{
		Config: cfg,
		Net:    plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)

	coll := s.client.Database("shellcn").Collection("people")
	_ = coll.Drop(ctx)
	if _, err := coll.InsertOne(ctx, bson.M{"_id": "ada", "name": "Ada", "role": "admin"}); err != nil {
		t.Fatalf("seed MongoDB: %v", err)
	}

	rc := plugin.NewRequestContext(ctx, plugin.User{ID: "u1", Username: "admin"}, s, nil, nil, nil)
	status, err := serverStatus(rc)
	if err != nil {
		t.Fatalf("server status: %v", err)
	}
	if status.(plugin.TableRow)["status"] != "healthy" {
		t.Fatalf("unexpected server status: %#v", status)
	}
	server, err := serverList(rc)
	if err != nil {
		t.Fatalf("server list: %v", err)
	}
	if len(server.(plugin.Page[plugin.TableRow]).Items) != 1 {
		t.Fatalf("unexpected server list: %#v", server)
	}
	health, err := healthList(rc)
	if err != nil {
		t.Fatalf("health list: %v", err)
	}
	if health.(plugin.Page[plugin.TableRow]).Total == nil {
		t.Fatalf("health list should report total: %#v", health)
	}
	ops, err := listCurrentOps(rc)
	if err != nil {
		t.Fatalf("current ops: %v", err)
	}
	if ops.(plugin.Page[plugin.TableRow]).Total == nil {
		t.Fatalf("current ops should report total: %#v", ops)
	}

	databases, err := listDatabases(rc)
	if err != nil {
		t.Fatalf("list databases: %v", err)
	}
	if !pageHasName(databases.(plugin.Page[plugin.TableRow]), "shellcn") {
		t.Fatalf("shellcn database missing: %#v", databases)
	}

	docs, err := listDocuments(plugin.NewRequestContext(ctx, plugin.User{}, s, map[string]string{"database": "shellcn", "collection": "people"}, nil, nil))
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	page := docs.(plugin.Page[plugin.TableRow])
	if len(page.Items) != 1 || page.Items[0]["name"] != "Ada" {
		t.Fatalf("unexpected documents: %#v", page.Items)
	}
	result, err := executeCommandRequest(ctx, s, "shellcn", sqldb.QueryRequest{Query: `{"find":"people","filter":{"_id":"ada"},"limit":1}`})
	if err != nil {
		t.Fatalf("command: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("unexpected command result: %#v", result.Rows)
	}

	// Database create round-trip (a database is created with its first collection).
	if _, err := createDatabase(plugin.NewRequestContext(ctx, plugin.User{}, s, nil, nil, []byte(`{"name":"shellcn_it_db","collection":"seed"}`))); err != nil {
		t.Fatalf("create database: %v", err)
	}
	defer func() { _ = s.client.Database("shellcn_it_db").Drop(context.Background()) }()
	if dbs, err := listDatabases(rc); err != nil {
		t.Fatalf("list databases: %v", err)
	} else if !pageHasName(dbs.(plugin.Page[plugin.TableRow]), "shellcn_it_db") {
		t.Fatalf("created database missing: %#v", dbs)
	}

	// Collection create round-trip.
	if _, err := createCollection(plugin.NewRequestContext(ctx, plugin.User{}, s, map[string]string{"database": "shellcn"}, nil, []byte(`{"name":"shellcn_it_coll"}`))); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	defer func() { _ = s.client.Database("shellcn").Collection("shellcn_it_coll").Drop(context.Background()) }()
	collections, err := listCollections(plugin.NewRequestContext(ctx, plugin.User{}, s, map[string]string{"database": "shellcn"}, mustQuery("p.database=shellcn"), nil))
	if err != nil {
		t.Fatalf("list collections: %v", err)
	}
	if !pageHasName(collections.(plugin.Page[plugin.TableRow]), "shellcn_it_coll") {
		t.Fatalf("created collection missing: %#v", collections)
	}
	if !pageHasStatus(collections.(plugin.Page[plugin.TableRow]), "shellcn_it_coll", "ready") {
		t.Fatalf("created collection status missing: %#v", collections)
	}

	// Index create → list → drop round-trip.
	idxParams := map[string]string{"database": "shellcn", "collection": "people"}
	if _, err := createIndex(plugin.NewRequestContext(ctx, plugin.User{}, s, idxParams, nil, []byte(`{"keys":{"role":1},"name":"role_1"}`))); err != nil {
		t.Fatalf("create index: %v", err)
	}
	indexes, err := listIndexes(plugin.NewRequestContext(ctx, plugin.User{}, s, idxParams, nil, nil))
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	if !pageHasName(indexes.(plugin.Page[plugin.TableRow]), "role_1") {
		t.Fatalf("created index missing: %#v", indexes)
	}
	if !pageHasStatus(indexes.(plugin.Page[plugin.TableRow]), "role_1", "ready") {
		t.Fatalf("created index status missing: %#v", indexes)
	}
	dropParams := map[string]string{"database": "shellcn", "collection": "people", "name": "role_1"}
	if _, err := dropIndex(plugin.NewRequestContext(ctx, plugin.User{}, s, dropParams, nil, nil)); err != nil {
		t.Fatalf("drop index: %v", err)
	}
}

func TestMongoDBListDocumentsBoundedPagingIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_MONGODB_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_MONGODB_INTEGRATION=1 to run against MongoDB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cfg := integrationConfig(ctx, t)
	sess, err := connect(ctx, plugin.ConnectConfig{Config: cfg, Net: plugintest.DirectTransport()})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)

	const (
		database   = "shellcn_paging"
		collection = "docs"
		seeded     = 250
		limit      = 100
	)
	coll := s.client.Database(database).Collection(collection)
	_ = coll.Drop(ctx)
	t.Cleanup(func() { _ = s.client.Database(database).Drop(context.Background()) })

	want := make([]string, 0, seeded)
	batch := make([]any, 0, seeded)
	for i := 0; i < seeded; i++ {
		id := "doc-" + strconv.Itoa(1000+i)
		want = append(want, id)
		group := "b"
		if i%2 == 0 {
			group = "a"
		}
		batch = append(batch, bson.M{"_id": id, "group": group, "n": i})
	}
	if _, err := coll.InsertMany(ctx, batch); err != nil {
		t.Fatalf("seed documents: %v", err)
	}

	first := documentPage(ctx, t, s, database, collection, url.Values{"limit": {strconv.Itoa(limit)}})
	if len(first.Items) != limit {
		t.Fatalf("first page is not bounded: got %d items, want %d", len(first.Items), limit)
	}
	if !strings.HasPrefix(first.NextCursor, documentCursorPrefix) {
		t.Fatalf("first page cursor: got %q, want %q prefix", first.NextCursor, documentCursorPrefix)
	}
	if first.Total == nil {
		t.Fatal("unfiltered listing should report the estimated total")
	}
	if *first.Total != seeded {
		t.Fatalf("unfiltered total: got %d, want %d", *first.Total, seeded)
	}

	got, sizes := drainDocuments(ctx, t, s, database, collection, url.Values{"limit": {strconv.Itoa(limit)}})
	if diff := comparePagedIDs(got, want); diff != "" {
		t.Fatalf("keyset paging: %s", diff)
	}
	if len(sizes) != 3 || sizes[0] != limit || sizes[1] != limit || sizes[2] != seeded-2*limit {
		t.Fatalf("page sizes: got %v, want [%d %d %d]", sizes, limit, limit, seeded-2*limit)
	}

	filtered := url.Values{"limit": {"50"}, "filter": {`{"group":"a"}`}}
	filteredFirst := documentPage(ctx, t, s, database, collection, filtered)
	if len(filteredFirst.Items) != 50 {
		t.Fatalf("filtered page is not bounded: got %d items, want 50", len(filteredFirst.Items))
	}
	if filteredFirst.NextCursor == "" {
		t.Fatal("filtered page should carry a next cursor")
	}
	if filteredFirst.Total != nil {
		t.Fatalf("filtered listing must omit the total instead of counting, got %d", *filteredFirst.Total)
	}

	filteredWant := make([]string, 0, seeded/2)
	for i := 0; i < seeded; i += 2 {
		filteredWant = append(filteredWant, want[i])
	}
	filteredGot, filteredSizes := drainDocuments(ctx, t, s, database, collection, filtered)
	if diff := comparePagedIDs(filteredGot, filteredWant); diff != "" {
		t.Fatalf("filtered paging: %s", diff)
	}
	if len(filteredSizes) != 3 || filteredSizes[2] != 25 {
		t.Fatalf("filtered page sizes: got %v, want [50 50 25]", filteredSizes)
	}

	exact := url.Values{"limit": {"50"}, "filter": {`{"group":"a"}`}, "count": {"exact"}}
	exactPage := documentPage(ctx, t, s, database, collection, exact)
	if exactPage.Total == nil || *exactPage.Total != len(filteredWant) {
		t.Fatalf("explicit exact count: got %v, want %d", exactPage.Total, len(filteredWant))
	}
	if len(exactPage.Items) != 50 {
		t.Fatalf("exact count page is not bounded: got %d items, want 50", len(exactPage.Items))
	}
}

func documentPage(ctx context.Context, t *testing.T, s *Session, database, collection string, query url.Values) plugin.Page[plugin.TableRow] {
	t.Helper()
	params := map[string]string{"database": database, "collection": collection}
	res, err := listDocuments(plugin.NewRequestContext(ctx, plugin.User{}, s, params, query, nil))
	if err != nil {
		t.Fatalf("list documents %v: %v", query, err)
	}
	page, ok := res.(plugin.Page[plugin.TableRow])
	if !ok {
		t.Fatalf("unexpected list result type %T", res)
	}
	return page
}

func drainDocuments(ctx context.Context, t *testing.T, s *Session, database, collection string, query url.Values) ([]string, []int) {
	t.Helper()
	ids := []string{}
	sizes := []int{}
	cursor := ""
	for i := 0; ; i++ {
		if i > 20 {
			t.Fatal("paging did not terminate")
		}
		next := url.Values{}
		for key, vals := range query {
			next[key] = vals
		}
		if cursor != "" {
			next.Set("cursor", cursor)
		}
		page := documentPage(ctx, t, s, database, collection, next)
		sizes = append(sizes, len(page.Items))
		for _, item := range page.Items {
			id, ok := item["_id"].(string)
			if !ok {
				t.Fatalf("unexpected _id %#v", item["_id"])
			}
			ids = append(ids, id)
		}
		if page.NextCursor == "" {
			return ids, sizes
		}
		if page.NextCursor == cursor {
			t.Fatalf("cursor did not advance: %q", cursor)
		}
		cursor = page.NextCursor
	}
}

func comparePagedIDs(got, want []string) string {
	seen := map[string]int{}
	for _, id := range got {
		seen[id]++
		if seen[id] > 1 {
			return "duplicate id " + id + " across pages"
		}
	}
	if len(got) != len(want) {
		return "paged " + strconv.Itoa(len(got)) + " documents, want " + strconv.Itoa(len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			return "position " + strconv.Itoa(i) + ": got " + got[i] + ", want " + want[i]
		}
	}
	return ""
}

func mustQuery(raw string) url.Values {
	v, _ := url.ParseQuery(raw)
	return v
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if addr := os.Getenv("SHELLCN_MONGODB_ADDR"); addr != "" {
		host, portText, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("parse SHELLCN_MONGODB_ADDR: %v", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse MongoDB port: %v", err)
		}
		return map[string]any{"host": host, "port": port, "database": "admin", "tls_mode": "disable", "read_only": false}
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_MONGODB_ADDR is not set")
	}
	name := "shellcn-mongodb-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name, "-p", "127.0.0.1::27017", "mongo:7")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "27017/tcp")
	host, portText, err := net.SplitHostPort(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse docker port %q: %v", portText, err)
	}
	cfg := map[string]any{"host": host, "port": port, "database": "admin", "tls_mode": "disable", "read_only": false}
	deadline := time.Now().Add(35 * time.Second)
	for {
		sess, err := connect(ctx, plugin.ConnectConfig{
			Config: cfg,
			Net:    plugintest.DirectTransport(),
		})
		if err == nil {
			_ = sess.Close()
			return cfg
		}
		if time.Now().After(deadline) {
			t.Fatalf("MongoDB container did not become ready: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func run(ctx context.Context, t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func pageHasName(page plugin.Page[plugin.TableRow], name string) bool {
	for _, item := range page.Items {
		if item["name"] == name {
			return true
		}
	}
	return false
}

func pageHasStatus(page plugin.Page[plugin.TableRow], name, status string) bool {
	for _, item := range page.Items {
		if item["name"] == name && item["status"] == status {
			return true
		}
	}
	return false
}

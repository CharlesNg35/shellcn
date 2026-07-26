package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
	redisclient "github.com/redis/go-redis/v9"
)

func TestRedisPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_REDIS_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_REDIS_INTEGRATION=1 to run against Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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

	if err := s.client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush db: %v", err)
	}
	if err := s.client.Set(ctx, "shellcn:string", "hello", 0).Err(); err != nil {
		t.Fatalf("seed string: %v", err)
	}
	if err := s.client.HSet(ctx, "shellcn:hash", map[string]string{"name": "ada"}).Err(); err != nil {
		t.Fatalf("seed hash: %v", err)
	}

	rc := plugin.NewRequestContext(ctx, plugin.User{ID: "u1", Username: "admin"}, s, nil, nil, nil)
	list, err := listKeys(rc)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if !hasKey(list.(plugin.Page[keyEntry]), "shellcn:string") {
		t.Fatalf("seeded string key missing: %#v", list)
	}
	detail, err := readKey(plugin.NewRequestContext(ctx, plugin.User{}, s, map[string]string{"key": "shellcn:hash"}, nil, nil))
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	hash := detail.(keyDetail)
	if hash.Type != "hash" {
		t.Fatalf("expected hash detail, got %#v", hash)
	}
	result, err := executeCommandRequest(ctx, s, sqldb.QueryRequest{Query: "GET shellcn:string"})
	if err != nil {
		t.Fatalf("command: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][0] != "hello" {
		t.Fatalf("unexpected command result: %#v", result.Rows)
	}

	// Key write → verify → delete → verify, through the route handlers.
	writeBody, _ := json.Marshal(map[string]any{"type": "string", "value": "world"})
	if _, err := writeKey(plugin.NewRequestContext(ctx, plugin.User{}, s, map[string]string{"key": "shellcn:written"}, nil, writeBody)); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if got, err := s.client.Get(ctx, "shellcn:written").Result(); err != nil || got != "world" {
		t.Fatalf("written key = %q, err %v", got, err)
	}
	if _, err := deleteKey(plugin.NewRequestContext(ctx, plugin.User{}, s, map[string]string{"key": "shellcn:written"}, nil, nil)); err != nil {
		t.Fatalf("delete key: %v", err)
	}
	if n, err := s.client.Exists(ctx, "shellcn:written").Result(); err != nil || n != 0 {
		t.Fatalf("deleted key still present (n=%d, err=%v)", n, err)
	}

	assertKeyPagingStaysBounded(ctx, t, s)
}

// assertKeyPagingStaysBounded seeds a keyspace far larger than one page and checks
// that a selective filter never makes a single request walk the whole keyspace and
// that following nextCursor still reaches the matching key.
func assertKeyPagingStaysBounded(ctx context.Context, t *testing.T, s *Session) {
	t.Helper()
	const seeded = 20_000
	pipe := s.client.Pipeline()
	for i := 0; i < seeded; i++ {
		pipe.Set(ctx, "bulk:"+strconv.Itoa(i), "v", 0)
		if (i+1)%1000 == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				t.Fatalf("seed bulk keys: %v", err)
			}
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("seed bulk keys: %v", err)
	}
	if err := s.client.Set(ctx, "bulk:needle:marker", "v", 0).Err(); err != nil {
		t.Fatalf("seed needle: %v", err)
	}

	const limit = 100
	maxItems := limit + s.opts.ScanCount
	pages, found := 0, false
	cursor := ""
	for {
		query := url.Values{"filter": {"needle"}, "limit": {strconv.Itoa(limit)}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		start := time.Now()
		result, err := listKeys(plugin.NewRequestContext(ctx, plugin.User{}, s, nil, query, nil))
		if err != nil {
			t.Fatalf("list keys page %d: %v", pages, err)
		}
		elapsed := time.Since(start)
		page := result.(plugin.Page[keyEntry])
		if len(page.Items) > maxItems {
			t.Fatalf("page %d returned %d keys, want at most %d", pages, len(page.Items), maxItems)
		}
		if elapsed > s.opts.Timeout {
			t.Fatalf("page %d took %s, a bounded page must stay under the command timeout", pages, elapsed)
		}
		if hasKey(page, "bulk:needle:marker") {
			found = true
		}
		pages++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > seeded {
			t.Fatal("key paging did not terminate")
		}
	}
	if pages < 2 {
		t.Fatalf("selective filter over %d keys finished in %d page(s); scanning is not incremental", seeded, pages)
	}
	if !found {
		t.Fatal("paging to the end never returned the matching key")
	}
}

// TestRedisBoundedLoadingIntegration proves against a real server that neither the
// key browser nor the value reader ever pulls a whole collection: every read is
// capped at the protocol (SCAN/HSCAN/SSCAN COUNT, XRANGE/LRANGE/ZRANGE stops) and
// oversized values come back short with Truncated=true.
func TestRedisBoundedLoadingIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_REDIS_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_REDIS_INTEGRATION=1 to run against Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	const (
		valueLimit  = 50
		collection  = 400
		seededKeys  = 600
		keyPageSize = 100
	)

	cfg := integrationConfig(ctx, t)
	cfg["read_only"] = true
	cfg["value_limit"] = valueLimit
	cfg["scan_count"] = plugin.MaxPageLimit

	sess, err := connect(ctx, plugin.ConnectConfig{Config: cfg, Net: plugintest.DirectTransport()})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)
	if err := s.client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush db: %v", err)
	}
	seedBoundedFixtures(ctx, t, s, seededKeys, collection)

	routes := plugintest.RouteMap(routes())

	t.Run("key scan paging", func(t *testing.T) {
		route := routes["redis.keys.list"]
		seen := make(map[string]int, seededKeys)
		order := make([]string, 0, seededKeys)
		cursor := ""
		for page := 0; ; page++ {
			query := url.Values{"filter": {"page:"}, "limit": {strconv.Itoa(keyPageSize)}}
			if cursor != "" {
				query.Set("cursor", cursor)
			}
			result, err := route.Handle(plugin.NewRequestContext(ctx, plugin.User{}, s, nil, query, nil))
			if err != nil {
				t.Fatalf("page %d: %v", page, err)
			}
			items := result.(plugin.Page[keyEntry])
			if len(items.Items) >= seededKeys {
				t.Fatalf("page %d returned %d keys: the browser read the whole keyspace instead of one page", page, len(items.Items))
			}
			if len(items.Items) > keyPageSize+s.opts.ScanCount {
				t.Fatalf("page %d returned %d keys, want a page bounded by the requested limit %d", page, len(items.Items), keyPageSize)
			}
			if page == 0 {
				if items.NextCursor == "" {
					t.Fatalf("first page of %d keys ended paging with %d items: SCAN is not incremental", seededKeys, len(items.Items))
				}
				if len(items.Items) < keyPageSize {
					t.Fatalf("first page returned %d keys, want at least the requested limit %d", len(items.Items), keyPageSize)
				}
			}
			for _, item := range items.Items {
				if !strings.HasPrefix(item.Key, "page:") {
					t.Fatalf("page %d leaked a non-matching key %q", page, item.Key)
				}
				if seen[item.Key]++; seen[item.Key] > 1 {
					t.Fatalf("key %q was returned on more than one page: pages overlap", item.Key)
				}
				order = append(order, item.Key)
			}
			if items.NextCursor == "" {
				break
			}
			cursor = items.NextCursor
			if page > seededKeys {
				t.Fatal("key paging never terminated")
			}
		}
		if len(order) != seededKeys {
			t.Fatalf("paging covered %d keys, want all %d seeded keys", len(order), seededKeys)
		}
		for i := 0; i < seededKeys; i++ {
			if key := boundedKeyName(i); seen[key] != 1 {
				t.Fatalf("key %q appeared %d times across all pages, want exactly once", key, seen[key])
			}
		}
	})

	read := func(t *testing.T, key string) (keyDetail, [][]string) {
		t.Helper()
		route := routes["redis.key.read"]
		var detail keyDetail
		sent := recordServerCommands(ctx, t, s, func() {
			result, err := route.Handle(plugin.NewRequestContext(ctx, plugin.User{}, s, map[string]string{"key": key}, nil, nil))
			if err != nil {
				t.Fatalf("read %s: %v", key, err)
			}
			detail = result.(keyDetail)
		})
		if !detail.Truncated {
			t.Fatalf("%s holds %d items but the reader did not report truncation", key, collection)
		}
		if detail.Size != collection {
			t.Fatalf("%s reported size %d, want the full server-side size %d", key, detail.Size, collection)
		}
		return detail, sent
	}

	t.Run("hash", func(t *testing.T) {
		detail, sent := read(t, "bounded:hash")
		fields := detail.Value.(map[string]string)
		if len(fields) == 0 || len(fields) > valueLimit {
			t.Fatalf("hash read returned %d fields, want 1..%d", len(fields), valueLimit)
		}
		if usedCommand(sent, "HGETALL") {
			t.Fatalf("hash read used HGETALL: %v", sentCommands(sent))
		}
		args := firstCommand(sent, "HSCAN")
		if args == nil || !hasArgPair(args, "count", strconv.Itoa(valueLimit)) {
			t.Fatalf("HSCAN args = %#v, want COUNT %d", args, valueLimit)
		}
		if n := countCommand(sent, "HSCAN"); n != 1 {
			t.Fatalf("hash read issued %d HSCAN round trips, want exactly one bounded page", n)
		}
	})

	t.Run("set", func(t *testing.T) {
		detail, sent := read(t, "bounded:set")
		members := detail.Value.([]string)
		if len(members) == 0 || len(members) > valueLimit {
			t.Fatalf("set read returned %d members, want 1..%d", len(members), valueLimit)
		}
		if usedCommand(sent, "SMEMBERS") {
			t.Fatalf("set read used SMEMBERS: %v", sentCommands(sent))
		}
		args := firstCommand(sent, "SSCAN")
		if args == nil || !hasArgPair(args, "count", strconv.Itoa(valueLimit)) {
			t.Fatalf("SSCAN args = %#v, want COUNT %d", args, valueLimit)
		}
		if n := countCommand(sent, "SSCAN"); n != 1 {
			t.Fatalf("set read issued %d SSCAN round trips, want exactly one bounded page", n)
		}
	})

	t.Run("stream", func(t *testing.T) {
		detail, sent := read(t, "bounded:stream")
		entries := detail.Value.([]map[string]any)
		if len(entries) != valueLimit {
			t.Fatalf("stream read returned %d entries, want exactly the limit %d", len(entries), valueLimit)
		}
		args := firstCommand(sent, "XRANGE")
		if args == nil || !hasArgPair(args, "count", strconv.Itoa(valueLimit+1)) {
			t.Fatalf("XRANGE args = %#v, want COUNT %d", args, valueLimit+1)
		}
	})

	t.Run("list", func(t *testing.T) {
		detail, sent := read(t, "bounded:list")
		values := detail.Value.([]string)
		if len(values) != valueLimit {
			t.Fatalf("list read returned %d values, want exactly the limit %d", len(values), valueLimit)
		}
		args := firstCommand(sent, "LRANGE")
		if len(args) != 4 || args[3] != strconv.Itoa(valueLimit) {
			t.Fatalf("LRANGE args = %#v, want a stop index of %d", args, valueLimit)
		}
	})

	t.Run("zset", func(t *testing.T) {
		detail, sent := read(t, "bounded:zset")
		members := detail.Value.([]map[string]any)
		if len(members) != valueLimit {
			t.Fatalf("zset read returned %d members, want exactly the limit %d", len(members), valueLimit)
		}
		args := firstCommand(sent, "ZRANGE")
		if len(args) < 4 || args[3] != strconv.Itoa(valueLimit) {
			t.Fatalf("ZRANGE args = %#v, want a stop index of %d", args, valueLimit)
		}
	})
}

func boundedKeyName(i int) string { return fmt.Sprintf("page:%04d", i) }

func seedBoundedFixtures(ctx context.Context, t *testing.T, s *Session, keys, collection int) {
	t.Helper()
	pipe := s.client.Pipeline()
	flush := func() {
		if _, err := pipe.Exec(ctx); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	for i := 0; i < keys; i++ {
		pipe.Set(ctx, boundedKeyName(i), "v", 0)
		if (i+1)%200 == 0 {
			flush()
		}
	}
	for i := 0; i < collection; i++ {
		id := fmt.Sprintf("%04d", i)
		pipe.HSet(ctx, "bounded:hash", "field:"+id, "v")
		pipe.SAdd(ctx, "bounded:set", "member:"+id)
		pipe.RPush(ctx, "bounded:list", "item:"+id)
		pipe.ZAdd(ctx, "bounded:zset", redisclient.Z{Score: float64(i), Member: "member:" + id})
		pipe.XAdd(ctx, &redisclient.XAddArgs{Stream: "bounded:stream", ID: fmt.Sprintf("%d-1", i+1), Values: map[string]any{"n": id}})
		if (i+1)%100 == 0 {
			flush()
		}
	}
	flush()
	for _, key := range []string{"bounded:hash", "bounded:set", "bounded:list", "bounded:zset", "bounded:stream"} {
		size, err := keySize(ctx, s.client, key, s.client.Type(ctx, key).Val())
		if err != nil {
			t.Fatalf("size of %s: %v", key, err)
		}
		if size != int64(collection) {
			t.Fatalf("%s seeded with %d items, want %d", key, size, collection)
		}
	}
}

// recordServerCommands captures every command the server executed while fn ran by
// temporarily logging all commands to the slowlog, which records full arguments.
func recordServerCommands(ctx context.Context, t *testing.T, s *Session, fn func()) [][]string {
	t.Helper()
	if err := s.client.Do(ctx, "SLOWLOG", "RESET").Err(); err != nil {
		t.Fatalf("slowlog reset: %v", err)
	}
	if err := s.client.ConfigSet(ctx, "slowlog-log-slower-than", "0").Err(); err != nil {
		t.Skipf("server does not allow slowlog capture: %v", err)
	}
	defer func() { _ = s.client.ConfigSet(ctx, "slowlog-log-slower-than", "10000").Err() }()
	fn()
	if err := s.client.ConfigSet(ctx, "slowlog-log-slower-than", "10000").Err(); err != nil {
		t.Fatalf("restore slowlog threshold: %v", err)
	}
	entries, err := s.client.SlowLogGet(ctx, 128).Result()
	if err != nil {
		t.Fatalf("slowlog get: %v", err)
	}
	out := make([][]string, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		if len(entries[i].Args) > 0 {
			out = append(out, entries[i].Args)
		}
	}
	return out
}

func firstCommand(sent [][]string, name string) []string {
	for _, args := range sent {
		if strings.EqualFold(args[0], name) {
			return args
		}
	}
	return nil
}

func countCommand(sent [][]string, name string) int {
	total := 0
	for _, args := range sent {
		if strings.EqualFold(args[0], name) {
			total++
		}
	}
	return total
}

func usedCommand(sent [][]string, name string) bool { return firstCommand(sent, name) != nil }

func sentCommands(sent [][]string) []string {
	out := make([]string, 0, len(sent))
	for _, args := range sent {
		out = append(out, strings.Join(args, " "))
	}
	return out
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if addr := os.Getenv("SHELLCN_REDIS_ADDR"); addr != "" {
		host, portText, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("parse SHELLCN_REDIS_ADDR: %v", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse Redis port: %v", err)
		}
		return map[string]any{"host": host, "port": port, "database": 0, "tls_mode": "disable", "read_only": false}
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_REDIS_ADDR is not set")
	}
	name := "shellcn-redis-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name, "-p", "127.0.0.1::6379", "redis:7-alpine")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "6379/tcp")
	host, portText, err := net.SplitHostPort(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse docker port %q: %v", portText, err)
	}
	cfg := map[string]any{"host": host, "port": port, "database": 0, "tls_mode": "disable", "read_only": false}
	deadline := time.Now().Add(20 * time.Second)
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
			t.Fatalf("Redis container did not become ready: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
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

func hasKey(page plugin.Page[keyEntry], key string) bool {
	for _, item := range page.Items {
		if item.Key == key {
			return true
		}
	}
	return false
}

package redis

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
)

type fakeRedis struct {
	ln       net.Listener
	replies  map[string]string
	mu       sync.Mutex
	commands [][]string
	wg       sync.WaitGroup
}

func newFakeRedis(t *testing.T, replies map[string]string) *fakeRedis {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeRedis{ln: ln, replies: replies}
	f.wg.Add(1)
	go f.serve()
	t.Cleanup(func() {
		_ = ln.Close()
		f.wg.Wait()
	})
	return f
}

func (f *fakeRedis) serve() {
	defer f.wg.Done()
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			defer func() { _ = conn.Close() }()
			f.handle(conn)
		}()
	}
}

func (f *fakeRedis) handle(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		args, err := readRESPCommand(reader)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.commands = append(f.commands, args)
		f.mu.Unlock()
		name := strings.ToUpper(args[0])
		reply, ok := f.replies[name]
		switch {
		case ok:
		case name == "HELLO":
			reply = "-ERR unknown command 'HELLO'\r\n"
		default:
			reply = "+OK\r\n"
		}
		if _, err := io.WriteString(conn, reply); err != nil {
			return
		}
	}
}

func (f *fakeRedis) sent(name string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, args := range f.commands {
		if strings.EqualFold(args[0], name) {
			return args
		}
	}
	return nil
}

func readRESPCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("unexpected request line %q", line)
	}
	count, err := strconv.Atoi(line[1:])
	if err != nil || count <= 0 {
		return nil, fmt.Errorf("unexpected argument count %q", line)
	}
	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		head, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		head = strings.TrimRight(head, "\r\n")
		size, err := strconv.Atoi(strings.TrimPrefix(head, "$"))
		if err != nil || size < 0 {
			return nil, fmt.Errorf("unexpected bulk header %q", head)
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:size]))
	}
	return args, nil
}

func respBulk(value string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)
}

func respBulkArray(values ...string) string {
	out := fmt.Sprintf("*%d\r\n", len(values))
	for _, value := range values {
		out += respBulk(value)
	}
	return out
}

func respArray(items ...string) string {
	return fmt.Sprintf("*%d\r\n", len(items)) + strings.Join(items, "")
}

func respScan(cursor string, values ...string) string {
	return respArray(respBulk(cursor), respBulkArray(values...))
}

func fakeSession(t *testing.T, f *fakeRedis, valueLimit int) (*Session, func()) {
	t.Helper()
	client := redisclient.NewClient(&redisclient.Options{
		Addr:         f.ln.Addr().String(),
		PoolSize:     1,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	return &Session{client: client, opts: options{ValueLimit: valueLimit}}, func() { _ = client.Close() }
}

func TestReadValueBoundsHashAtProtocol(t *testing.T) {
	f := newFakeRedis(t, map[string]string{
		"HSCAN": respScan("12", "a", "1", "b", "2", "c", "3", "d", "4"),
	})
	s, closeClient := fakeSession(t, f, 3)
	defer closeClient()

	value, truncated, err := readValue(context.Background(), s, s.client, "h", "hash")
	if err != nil {
		t.Fatalf("read hash: %v", err)
	}
	fields, ok := value.(map[string]string)
	if !ok {
		t.Fatalf("hash value = %T, want map[string]string", value)
	}
	if len(fields) != 3 || fields["a"] != "1" || fields["c"] != "3" {
		t.Fatalf("hash fields = %#v, want the first 3 pairs", fields)
	}
	if !truncated {
		t.Fatal("a hash larger than the value limit must report truncation")
	}
	if got := f.sent("HGETALL"); got != nil {
		t.Fatalf("hash read must not use HGETALL: %#v", got)
	}
	args := f.sent("HSCAN")
	if len(args) == 0 {
		t.Fatal("hash read must scan with a bounded COUNT")
	}
	if !hasArgPair(args, "count", "3") {
		t.Fatalf("HSCAN args = %#v, want COUNT 3", args)
	}
}

func TestReadValueBoundsSetAtProtocol(t *testing.T) {
	f := newFakeRedis(t, map[string]string{
		"SSCAN": respScan("0", "e", "d", "c", "b", "a"),
	})
	s, closeClient := fakeSession(t, f, 3)
	defer closeClient()

	value, truncated, err := readValue(context.Background(), s, s.client, "s", "set")
	if err != nil {
		t.Fatalf("read set: %v", err)
	}
	members, ok := value.([]string)
	if !ok || len(members) != 3 {
		t.Fatalf("set value = %#v, want 3 members", value)
	}
	if members[0] != "c" || members[2] != "e" {
		t.Fatalf("set members = %#v, want the page sorted", members)
	}
	if !truncated {
		t.Fatal("a set larger than the value limit must report truncation")
	}
	if got := f.sent("SMEMBERS"); got != nil {
		t.Fatalf("set read must not use SMEMBERS: %#v", got)
	}
	if args := f.sent("SSCAN"); !hasArgPair(args, "count", "3") {
		t.Fatalf("SSCAN args = %#v, want COUNT 3", args)
	}
}

func TestReadValueBoundsStreamAtProtocol(t *testing.T) {
	entry := func(id string, values ...string) string {
		return respArray(respBulk(id), respBulkArray(values...))
	}
	f := newFakeRedis(t, map[string]string{
		"XRANGE": respArray(
			entry("1-1", "f", "1"),
			entry("2-1", "f", "2"),
			entry("3-1", "f", "3"),
			entry("4-1", "f", "4"),
		),
	})
	s, closeClient := fakeSession(t, f, 3)
	defer closeClient()

	value, truncated, err := readValue(context.Background(), s, s.client, "x", "stream")
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	entries, ok := value.([]map[string]any)
	if !ok || len(entries) != 3 {
		t.Fatalf("stream value = %#v, want 3 entries", value)
	}
	if entries[0]["id"] != "1-1" {
		t.Fatalf("stream entries = %#v, want the oldest first", entries)
	}
	if !truncated {
		t.Fatal("a stream larger than the value limit must report truncation")
	}
	if args := f.sent("XRANGE"); !hasArgPair(args, "count", "4") {
		t.Fatalf("XRANGE args = %#v, want COUNT 4 (limit + 1)", args)
	}
}

func TestReadValueKeepsSmallCollectionsWhole(t *testing.T) {
	f := newFakeRedis(t, map[string]string{
		"HSCAN":  respScan("0", "a", "1", "b", "2"),
		"SSCAN":  respScan("0", "b", "a"),
		"LRANGE": respBulkArray("x", "y"),
		"XRANGE": respArray(respArray(respBulk("1-1"), respBulkArray("f", "1"))),
	})
	s, closeClient := fakeSession(t, f, 3)
	defer closeClient()

	for _, tc := range []struct{ kind string }{{"hash"}, {"set"}, {"list"}, {"stream"}} {
		value, truncated, err := readValue(context.Background(), s, s.client, "k", tc.kind)
		if err != nil {
			t.Fatalf("read %s: %v", tc.kind, err)
		}
		if truncated {
			t.Fatalf("%s below the value limit must not report truncation: %#v", tc.kind, value)
		}
	}
	if args := f.sent("LRANGE"); len(args) != 4 || args[3] != "3" {
		t.Fatalf("LRANGE args = %#v, want a stop index of limit", args)
	}
}

func hasArgPair(args []string, name, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if strings.EqualFold(args[i], name) && args[i+1] == value {
			return true
		}
	}
	return false
}

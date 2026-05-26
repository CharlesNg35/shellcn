package recording

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func parseLines(t *testing.T, data []byte) (header map[string]any, events [][]any) {
	t.Helper()
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if err := json.Unmarshal(lines[0], &header); err != nil {
		t.Fatalf("header parse: %v", err)
	}
	for _, ln := range lines[1:] {
		var ev []any
		if err := json.Unmarshal(ln, &ev); err != nil {
			t.Fatalf("event parse %q: %v", ln, err)
		}
		events = append(events, ev)
	}
	return header, events
}

func TestAsciicastHeaderAllowlistsEnv(t *testing.T) {
	var buf bytes.Buffer
	_, err := NewAsciicastRecorder(&buf, StartInfo{
		Cols: 120, Rows: 40, Title: "demo", Start: time.Unix(1700000000, 0),
		Env: map[string]string{"TERM": "xterm-256color", "SECRET": "leak", "SHELL": "/bin/bash"},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	header, _ := parseLines(t, buf.Bytes())
	if header["version"] != float64(2) || header["width"] != float64(120) || header["height"] != float64(40) {
		t.Fatalf("bad header: %+v", header)
	}
	if header["idle_time_limit"] != float64(2) {
		t.Fatalf("idle time limit missing: %+v", header)
	}
	if header["timestamp"] != float64(1700000000) || header["title"] != "demo" {
		t.Fatalf("missing timestamp/title: %+v", header)
	}
	env, _ := header["env"].(map[string]any)
	if env["TERM"] != "xterm-256color" || env["SHELL"] != "/bin/bash" {
		t.Errorf("allowed env missing: %+v", env)
	}
	if _, leaked := env["SECRET"]; leaked {
		t.Error("non-allowlisted env leaked into recording header")
	}
}

func TestAsciicastEncodesEventsAndResize(t *testing.T) {
	var buf bytes.Buffer
	r, _ := NewAsciicastRecorder(&buf, StartInfo{})
	_ = r.WriteOutput(500*time.Millisecond, []byte("hello"))
	_ = r.Resize(1200*time.Millisecond, 132, 43)
	_, events := parseLines(t, buf.Bytes())

	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[0][1] != "o" || events[0][2] != "hello" || events[0][0] != 0.5 {
		t.Errorf("output event: %+v", events[0])
	}
	if events[1][1] != "r" || events[1][2] != "132x43" {
		t.Errorf("resize event must be {cols}x{rows}: %+v", events[1])
	}
}

func TestAsciicastOmitsInputUnlessWritten(t *testing.T) {
	var buf bytes.Buffer
	r, _ := NewAsciicastRecorder(&buf, StartInfo{})
	_ = r.WriteOutput(time.Second, []byte("out"))
	if strings.Contains(buf.String(), `"i"`) {
		t.Errorf("input events must be absent unless explicitly recorded: %s", buf.String())
	}
}

func TestAsciicastGolden(t *testing.T) {
	var buf bytes.Buffer
	r, err := NewAsciicastRecorder(&buf, StartInfo{
		Cols: 80, Rows: 24, Title: "golden", Start: time.Unix(1700000000, 0),
		Env: map[string]string{"TERM": "xterm-256color"},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_ = r.WriteOutput(100*time.Millisecond, []byte("$ echo hi\r\n"))
	_ = r.WriteOutput(250*time.Millisecond, []byte("hi\r\n"))
	_ = r.Resize(300*time.Millisecond, 100, 30)
	_ = r.WriteOutput(420*time.Millisecond, []byte("$ "))
	_ = r.Close()

	got := buf.Bytes()
	goldenPath := filepath.Join("testdata", "sample.cast")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("asciicast drifted from golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

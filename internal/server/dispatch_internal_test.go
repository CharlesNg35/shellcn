package server

import (
	"net"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestStreamKindHasContinuousClientReader(t *testing.T) {
	tests := []struct {
		name string
		kind plugin.StreamKind
		want bool
	}{
		{name: "terminal", kind: plugin.StreamTerminal, want: true},
		{name: "desktop", kind: plugin.StreamDesktop, want: true},
		{name: "canvas", kind: plugin.StreamCanvas, want: true},
		{name: "logs", kind: plugin.StreamLogs, want: false},
		{name: "query", kind: plugin.StreamQuery, want: false},
		{name: "metrics", kind: plugin.StreamMetrics, want: false},
		{name: "file_job", kind: plugin.StreamFileJob, want: false},
		{name: "unknown", kind: plugin.StreamKind("custom"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := streamKindHasContinuousClientReader(tt.kind); got != tt.want {
				t.Fatalf("streamKindHasContinuousClientReader(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestStreamKindKeepAlivePolicy(t *testing.T) {
	tests := []struct {
		name          string
		kind          plugin.StreamKind
		enabled       bool
		controlReader bool
	}{
		{name: "terminal", kind: plugin.StreamTerminal, enabled: true},
		{name: "desktop", kind: plugin.StreamDesktop, enabled: true},
		{name: "canvas", kind: plugin.StreamCanvas, enabled: true},
		{name: "logs", kind: plugin.StreamLogs, enabled: true, controlReader: true},
		{name: "query", kind: plugin.StreamQuery, enabled: true},
		{name: "metrics", kind: plugin.StreamMetrics},
		{name: "file_job", kind: plugin.StreamFileJob, enabled: true},
		{name: "unknown", kind: plugin.StreamKind("custom")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := streamKindKeepAlivePolicy(tt.kind)
			if got.enabled != tt.enabled || got.controlReader != tt.controlReader {
				t.Fatalf("streamKindKeepAlivePolicy(%q) = %+v, want enabled=%v controlReader=%v", tt.kind, got, tt.enabled, tt.controlReader)
			}
		})
	}
}

func TestActiveConnTracksReadAndWriteActivity(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	tracked := newActiveConn(server)
	initial := tracked.LastActive()
	if initial.IsZero() {
		t.Fatal("initial activity should be recorded")
	}

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4)
		if _, err := tracked.Read(buf); err != nil {
			t.Errorf("read: %v", err)
		}
	}()
	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	<-readDone
	afterRead := tracked.LastActive()
	if afterRead.Before(initial) {
		t.Fatalf("read activity went backwards: %v before %v", afterRead, initial)
	}

	time.Sleep(time.Millisecond)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		if _, err := tracked.Write([]byte("pong")); err != nil {
			t.Errorf("write: %v", err)
		}
	}()
	buf := make([]byte, 4)
	if _, err := client.Read(buf); err != nil {
		t.Fatalf("client read: %v", err)
	}
	<-writeDone
	if !tracked.LastActive().After(afterRead) {
		t.Fatalf("write activity was not updated: %v after %v", tracked.LastActive(), afterRead)
	}
}

func TestActiveConnTreatsBlockedWriteAsActive(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	tracked := newActiveConn(server)
	beforeWrite := tracked.LastActive()

	writeStarted := make(chan struct{})
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		close(writeStarted)
		_, _ = tracked.Write([]byte("blocked"))
	}()
	<-writeStarted

	deadline := time.Now().Add(time.Second)
	for !tracked.LastActive().After(beforeWrite) {
		if time.Now().After(deadline) {
			t.Fatal("blocked write was not marked active")
		}
		time.Sleep(time.Millisecond)
	}

	buf := make([]byte, 7)
	if _, err := client.Read(buf); err != nil {
		t.Fatalf("client read: %v", err)
	}
	<-writeDone
}

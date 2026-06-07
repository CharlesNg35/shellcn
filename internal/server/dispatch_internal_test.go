package server

import (
	"testing"

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
		{name: "metrics", kind: plugin.StreamMetrics, want: false},
		{name: "file", kind: plugin.StreamFile, want: false},
		{name: "unknown", kind: plugin.StreamKind("query"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := streamKindHasContinuousClientReader(tt.kind); got != tt.want {
				t.Fatalf("streamKindHasContinuousClientReader(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

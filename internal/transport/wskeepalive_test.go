package transport

import (
	"testing"
	"time"
)

func TestShouldPing(t *testing.T) {
	now := time.Unix(100, 0)
	idleFor := 25 * time.Second

	tests := []struct {
		name       string
		lastActive time.Time
		want       bool
	}{
		{name: "never active", lastActive: time.Time{}, want: true},
		{name: "recently active", lastActive: now.Add(-idleFor + time.Second), want: false},
		{name: "exactly idle interval", lastActive: now.Add(-idleFor), want: true},
		{name: "older than idle interval", lastActive: now.Add(-idleFor - time.Second), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPing(tt.lastActive, now, idleFor); got != tt.want {
				t.Fatalf("shouldPing = %v, want %v", got, tt.want)
			}
		})
	}
}

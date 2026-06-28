package server

import (
	"strings"
	"testing"
)

func TestCleanWorkspaceQuery(t *testing.T) {
	if got := cleanWorkspaceQuery("v=detail:pod:uid:n=api,ns=default\r\n"); got != "?v=detail:pod:uid:n=api,ns=default" {
		t.Fatalf("query = %q", got)
	}
	if got := cleanWorkspaceQuery(""); got != "" {
		t.Fatalf("empty query = %q", got)
	}
	long := "?" + strings.Repeat("a", 3000)
	if got := cleanWorkspaceQuery(long); len(got) != 2048 {
		t.Fatalf("truncated length = %d, want 2048", len(got))
	}
}

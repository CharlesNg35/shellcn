package filesystem

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// pagedFS is a memFS whose listings are cursor-native: it records what the
// browser asked for so a page request can be proved bounded.
type pagedFS struct {
	*memFS
	total      int
	readDirHit int
	gotCursor  string
	gotLimit   int
}

func (p *pagedFS) Filesystem() (Client, error) { return p, nil }

func (p *pagedFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	p.readDirHit++
	return p.memFS.ReadDir(ctx, path)
}

func (p *pagedFS) ReadDirPage(_ context.Context, _ string, cursor string, limit int) ([]os.FileInfo, string, error) {
	p.gotCursor = cursor
	p.gotLimit = limit
	offset := 0
	if cursor != "" {
		if _, err := fmt.Sscanf(cursor, "%d", &offset); err != nil {
			return nil, "", err
		}
	}
	end := offset + limit
	next := ""
	if end < p.total {
		next = fmt.Sprintf("%d", end)
	} else {
		end = p.total
	}
	infos := make([]os.FileInfo, 0, end-offset)
	for i := offset; i < end; i++ {
		infos = append(infos, memInfo{name: fmt.Sprintf("obj-%06d", i)})
	}
	return infos, next, nil
}

func listRoute(t *testing.T) plugin.Route {
	t.Helper()
	for _, r := range Routes("test", "test") {
		if r.ID == "test.files.list" {
			return r
		}
	}
	t.Fatal("list route missing")
	return plugin.Route{}
}

func TestListUsesCursorNativePaging(t *testing.T) {
	fs := &pagedFS{memFS: newMemFS(), total: 1_000_000}
	route := listRoute(t)

	out, err := route.Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, fs, map[string]string{"path": "/"}, nil, nil))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	page, ok := out.(FilePage)
	if !ok {
		t.Fatalf("list returned %T", out)
	}
	if fs.readDirHit != 0 {
		t.Fatalf("cursor-native backend must not be drained through ReadDir (%d calls)", fs.readDirHit)
	}
	if fs.gotLimit != plugin.DefaultPageLimit || fs.gotCursor != "" {
		t.Fatalf("first page asked for cursor %q limit %d", fs.gotCursor, fs.gotLimit)
	}
	if len(page.Items) != plugin.DefaultPageLimit {
		t.Fatalf("page holds %d entries, want %d", len(page.Items), plugin.DefaultPageLimit)
	}
	if page.NextCursor == "" || !page.Truncated {
		t.Fatalf("a truncated listing must advertise more entries: %+v", page)
	}
	if page.Total != nil {
		t.Fatalf("total must be omitted for cursor-native listings, got %d", *page.Total)
	}
	if page.Items[0].Path != "/obj-000000" {
		t.Fatalf("entry path = %q", page.Items[0].Path)
	}

	query := map[string][]string{"cursor": {page.NextCursor}, "limit": {"999999"}}
	out, err = route.Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, fs, map[string]string{"path": "/"}, query, nil))
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if fs.gotCursor != page.NextCursor {
		t.Fatalf("cursor %q was not forwarded to the backend (got %q)", page.NextCursor, fs.gotCursor)
	}
	if fs.gotLimit != plugin.MaxPageLimit {
		t.Fatalf("limit must clamp to %d, got %d", plugin.MaxPageLimit, fs.gotLimit)
	}
	if got := len(out.(FilePage).Items); got != plugin.MaxPageLimit {
		t.Fatalf("clamped page holds %d entries, want %d", got, plugin.MaxPageLimit)
	}
}

func TestListFallsBackToReadDir(t *testing.T) {
	fs := newMemFS()
	fs.files["/b.txt"] = []byte("b")
	fs.files["/a.txt"] = []byte("a")
	out, err := listRoute(t).Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, fs, map[string]string{"path": "/"}, nil, nil))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	page := out.(FilePage)
	if page.Total == nil || *page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("bounded backends keep their exact total: %+v", page)
	}
	if page.NextCursor != "" || page.Truncated {
		t.Fatalf("a complete listing must not advertise more entries: %+v", page)
	}
	if page.Items[0].Name != "a.txt" {
		t.Fatalf("entries must stay sorted: %+v", page.Items)
	}
}

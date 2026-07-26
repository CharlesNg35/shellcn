package s3compat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// fakeS3 replays canned listing XML page by page (the last body repeats) and
// records every request, so a handler can be proved to issue exactly one bounded
// call, or to interleave listing with work instead of draining first.
type fakeS3 struct {
	mu       sync.Mutex
	bodies   []string
	served   int
	requests []recordedRequest
}

type recordedRequest struct {
	method string
	query  url.Values
}

func (f *fakeS3) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{method: r.Method, query: r.URL.Query()})
		body := deleteResult
		if _, isDelete := r.URL.Query()["delete"]; !isDelete {
			body = f.bodies[f.served]
			if f.served < len(f.bodies)-1 {
				f.served++
			}
		}
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(body))
	})
}

func (f *fakeS3) requested() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests
}

func (f *fakeS3) calls() []url.Values {
	queries := make([]url.Values, 0, len(f.requested()))
	for _, r := range f.requested() {
		queries = append(queries, r.query)
	}
	return queries
}

func newFakeClient(t *testing.T, bodies ...string) (*Client, *fakeS3) {
	t.Helper()
	fake := &fakeS3{bodies: bodies}
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)
	api := awss3.New(awss3.Options{
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("id", "secret", ""),
		BaseEndpoint: aws.String(srv.URL),
		UsePathStyle: true,
	})
	return &Client{s3: api, bucket: "bucket"}, fake
}

const listObjectsPage = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>bucket</Name>
  <Prefix>dir/</Prefix>
  <Delimiter>/</Delimiter>
  <MaxKeys>2</MaxKeys>
  <KeyCount>3</KeyCount>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>token-2</NextContinuationToken>
  <Contents><Key>dir/</Key><Size>0</Size><LastModified>2026-01-01T00:00:00.000Z</LastModified></Contents>
  <Contents><Key>dir/report.txt</Key><Size>7</Size><LastModified>2026-01-02T00:00:00.000Z</LastModified></Contents>
  <CommonPrefixes><Prefix>dir/nested/</Prefix></CommonPrefixes>
</ListBucketResult>`

func TestReadDirPageIssuesOneBoundedRequest(t *testing.T) {
	c, fake := newFakeClient(t, listObjectsPage)

	infos, next, err := c.ReadDirPage(context.Background(), "/dir", "token-1", 2)
	if err != nil {
		t.Fatalf("ReadDirPage: %v", err)
	}
	calls := fake.calls()
	if len(calls) != 1 {
		t.Fatalf("want exactly one ListObjectsV2 call, got %d", len(calls))
	}
	q := calls[0]
	if q.Get("max-keys") != "2" {
		t.Fatalf("max-keys = %q, want the page limit", q.Get("max-keys"))
	}
	if q.Get("continuation-token") != "token-1" {
		t.Fatalf("continuation-token = %q, want the page cursor", q.Get("continuation-token"))
	}
	if q.Get("delimiter") != "/" || q.Get("prefix") != "dir/" {
		t.Fatalf("listing must stay scoped to the directory: %v", q)
	}
	if next != "token-2" {
		t.Fatalf("next cursor = %q, want the continuation token", next)
	}
	if len(infos) != 2 || !infos[0].IsDir() || infos[0].Name() != "nested" || infos[1].Name() != "report.txt" {
		t.Fatalf("page entries = %+v, want the folder first then the object", infos)
	}
}

func TestReadDirPageClampsLimit(t *testing.T) {
	c, fake := newFakeClient(t, listObjectsPage)
	if _, _, err := c.ReadDirPage(context.Background(), "/dir", "", plugin.MaxPageLimit*10); err != nil {
		t.Fatalf("ReadDirPage: %v", err)
	}
	if got := fake.calls()[0].Get("max-keys"); got != "500" {
		t.Fatalf("max-keys = %q, want the clamped %d", got, plugin.MaxPageLimit)
	}
	if _, _, err := c.ReadDirPage(context.Background(), "/dir", "", 0); err != nil {
		t.Fatalf("ReadDirPage: %v", err)
	}
	if got := fake.calls()[1].Get("max-keys"); got != "50" {
		t.Fatalf("max-keys = %q, want the default %d", got, plugin.DefaultPageLimit)
	}
	if fake.calls()[0].Get("continuation-token") != "" {
		t.Fatal("an empty cursor must not send a continuation token")
	}
}

const deleteResult = `<?xml version="1.0" encoding="UTF-8"?>
<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></DeleteResult>`

const flatPageOne = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>bucket</Name>
  <Prefix>dir/</Prefix>
  <KeyCount>2</KeyCount>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>token-2</NextContinuationToken>
  <Contents><Key>dir/a.txt</Key><Size>1</Size><LastModified>2026-01-01T00:00:00.000Z</LastModified></Contents>
  <Contents><Key>dir/b.txt</Key><Size>1</Size><LastModified>2026-01-01T00:00:00.000Z</LastModified></Contents>
</ListBucketResult>`

const flatPageTwo = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>bucket</Name>
  <Prefix>dir/</Prefix>
  <KeyCount>1</KeyCount>
  <IsTruncated>false</IsTruncated>
  <Contents><Key>dir/c.txt</Key><Size>1</Size><LastModified>2026-01-01T00:00:00.000Z</LastModified></Contents>
</ListBucketResult>`

func TestDeletePrefixStreamsPages(t *testing.T) {
	c, fake := newFakeClient(t, flatPageOne, flatPageTwo)
	if err := c.deletePrefix(context.Background(), "dir/"); err != nil {
		t.Fatalf("deletePrefix: %v", err)
	}
	got := make([]string, 0, len(fake.requested()))
	for _, r := range fake.requested() {
		if _, isDelete := r.query["delete"]; isDelete {
			got = append(got, "delete")
			continue
		}
		got = append(got, "list")
	}
	want := []string{"list", "delete", "list", "delete"}
	if len(got) != len(want) {
		t.Fatalf("request sequence = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("request sequence = %v, want %v (keys must be deleted per page, not collected first)", got, want)
		}
	}
	if got := fake.calls()[2].Get("continuation-token"); got != "token-2" {
		t.Fatalf("second listing must resume from the continuation token, got %q", got)
	}
}

func TestDeletePrefixHonoursCancellation(t *testing.T) {
	c, fake := newFakeClient(t, flatPageOne)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.deletePrefix(ctx, "dir/"); err == nil {
		t.Fatal("a cancelled delete must stop instead of walking the prefix")
	}
	if n := len(fake.requested()); n != 0 {
		t.Fatalf("cancelled delete issued %d requests", n)
	}
}

func TestReadDirWalksEveryPage(t *testing.T) {
	c, fake := newFakeClient(t, listObjectsPage, flatPageTwo)
	infos, err := c.ReadDir(context.Background(), "/dir")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(fake.calls()) != 2 {
		t.Fatalf("ReadDir made %d calls, want one per page", len(fake.calls()))
	}
	if got := fake.calls()[0].Get("max-keys"); got != "500" {
		t.Fatalf("walk page size = %q, want the max page limit", got)
	}
	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.Name())
	}
	if len(names) != 3 || names[0] != "nested" || names[1] != "c.txt" || names[2] != "report.txt" {
		t.Fatalf("walk collected %v, want every page sorted with folders first", names)
	}
}

const listVersionsPage = `<?xml version="1.0" encoding="UTF-8"?>
<ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>bucket</Name>
  <MaxKeys>2</MaxKeys>
  <IsTruncated>true</IsTruncated>
  <NextKeyMarker>report.txt</NextKeyMarker>
  <NextVersionIdMarker>v9</NextVersionIdMarker>
  <Version><Key>report.txt</Key><VersionId>v2</VersionId><IsLatest>true</IsLatest><Size>7</Size><LastModified>2026-01-02T00:00:00.000Z</LastModified></Version>
  <DeleteMarker><Key>old.txt</Key><VersionId>v1</VersionId><IsLatest>false</IsLatest><LastModified>2026-01-01T00:00:00.000Z</LastModified></DeleteMarker>
</ListVersionsResult>`

func TestListVersionsPagesInOneRequest(t *testing.T) {
	c, fake := newFakeClient(t, listVersionsPage)
	route := adminRoute(t, "bucket.versions")

	out, err := route.Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, &Session{fs: c},
		map[string]string{"bucket": "bucket"}, url.Values{"limit": {"2"}}, nil))
	if err != nil {
		t.Fatalf("listVersions: %v", err)
	}
	page, ok := out.(plugin.Page[objectVersionEntry])
	if !ok {
		t.Fatalf("listVersions returned %T", out)
	}
	calls := fake.calls()
	if len(calls) != 1 {
		t.Fatalf("want exactly one ListObjectVersions call, got %d", len(calls))
	}
	if got := calls[0].Get("max-keys"); got != "2" {
		t.Fatalf("max-keys = %q, want the page limit", got)
	}
	if len(page.Items) != 2 || page.Items[0].Key != "report.txt" || !page.Items[1].DeleteMarker {
		t.Fatalf("page items = %+v, want the version then the delete marker", page.Items)
	}
	if page.Total != nil {
		t.Fatalf("total must be omitted, got %d", *page.Total)
	}
	if page.NextCursor == "" {
		t.Fatal("a truncated version listing must return a cursor")
	}

	if _, err := route.Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, &Session{fs: c},
		map[string]string{"bucket": "bucket"}, url.Values{"cursor": {page.NextCursor}}, nil)); err != nil {
		t.Fatalf("second page: %v", err)
	}
	q := fake.calls()[1]
	if q.Get("key-marker") != "report.txt" || q.Get("version-id-marker") != "v9" {
		t.Fatalf("cursor must resume from the returned markers: %v", q)
	}
}

func TestVersionCursorRoundTrip(t *testing.T) {
	cursor := encodeVersionCursor("dir/a b+c.txt", "v1")
	key, version := decodeVersionCursor(cursor)
	if key != "dir/a b+c.txt" || version != "v1" {
		t.Fatalf("round trip = %q, %q", key, version)
	}
	if encodeVersionCursor("", "") != "" {
		t.Fatal("an exhausted listing must encode to an empty cursor")
	}
	for _, bad := range []string{"not base64!!", "Zm9v"} {
		if k, v := decodeVersionCursor(bad); k != "" || v != "" {
			t.Fatalf("malformed cursor %q decoded to %q/%q", bad, k, v)
		}
	}
}

func adminRoute(t *testing.T, suffix string) plugin.Route {
	t.Helper()
	for _, r := range AdminRoutes("minio") {
		if r.ID == routeID("minio", suffix) {
			return r
		}
	}
	t.Fatalf("route %q missing", suffix)
	return plugin.Route{}
}

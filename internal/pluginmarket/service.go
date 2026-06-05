package pluginmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	maxIndexBytes  = 8 << 20
	refreshAfter   = 5 * time.Minute
	requestTimeout = 2 * time.Minute
)

// Service fetches and caches the registry index. Multiple URLs merge by plugin
// name, first wins — operators can front the official registry with their own.
type Service struct {
	urls   []string
	client *http.Client

	mu      sync.Mutex
	cache   map[string]*cachedIndex
	merged  []Entry
	fetched time.Time
}

type cachedIndex struct {
	etag  string
	index *Index
}

func New(urls []string) *Service {
	return &Service{
		urls:   urls,
		client: &http.Client{Timeout: requestTimeout},
		cache:  map[string]*cachedIndex{},
	}
}

// Entries returns the merged catalog; a URL that fails keeps serving its last
// good copy.
func (s *Service) Entries(ctx context.Context) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.fetched) < refreshAfter && s.merged != nil {
		return s.merged, nil
	}

	var errs []error
	seen := map[string]bool{}
	merged := make([]Entry, 0)
	for _, url := range s.urls {
		idx, err := s.fetchLocked(ctx, url)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", url, err))
			continue
		}
		for _, e := range idx.Plugins {
			if seen[e.Name] {
				continue
			}
			seen[e.Name] = true
			merged = append(merged, e)
		}
	}
	if len(merged) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("%w: market index unavailable: %v", plugin.ErrUnavailable, errs[0])
	}
	s.merged, s.fetched = merged, time.Now()
	return merged, nil
}

func (s *Service) Entry(ctx context.Context, name string) (Entry, error) {
	entries, err := s.Entries(ctx)
	if err != nil {
		return Entry{}, err
	}
	for _, e := range entries {
		if e.Name == name {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("%w: plugin %q is not in the market index", plugin.ErrNotFound, name)
}

func (s *Service) fetchLocked(ctx context.Context, url string) (*Index, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	cached := s.cache[url]
	if cached != nil && cached.etag != "" {
		req.Header.Set("If-None-Match", cached.etag)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if cached != nil {
			return cached.index, nil
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return cached.index, nil
	case http.StatusOK:
	default:
		if cached != nil {
			return cached.index, nil
		}
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxIndexBytes))
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}
	if idx.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported index schemaVersion %d", idx.SchemaVersion)
	}
	s.cache[url] = &cachedIndex{etag: resp.Header.Get("ETag"), index: &idx}
	return &idx, nil
}

// Package version reports the running build version and checks GitHub for a
// newer release. Development builds (a prerelease or non-semver version) never
// reach the network.
package version

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

const (
	releasesAPI  = "https://api.github.com/repos/CharlesNg35/shellcn/releases/latest"
	releasesPage = "https://github.com/CharlesNg35/shellcn/releases"

	checkTTL     = 6 * time.Hour
	refreshFloor = time.Minute
	httpTimeout  = 8 * time.Second
)

// Info is the version/update status returned to clients.
type Info struct {
	Current         string     `json:"current"`
	Dev             bool       `json:"dev"`
	Latest          string     `json:"latest,omitempty"`
	UpdateAvailable bool       `json:"updateAvailable"`
	ReleaseURL      string     `json:"releaseUrl,omitempty"`
	CheckDisabled   bool       `json:"checkDisabled,omitempty"`
	CheckedAt       *time.Time `json:"checkedAt,omitempty"`
	Error           string     `json:"error,omitempty"`
}

// IsDev reports whether v is a development build that must not check for updates.
// Only a clean release tag (vX.Y.Z, no prerelease or build metadata) checks.
func IsDev(v string) bool {
	v = strings.TrimSpace(v)
	return !semver.IsValid(v) || semver.Prerelease(v) != "" || semver.Build(v) != ""
}

// Checker resolves the latest release, caching the result to avoid hammering the
// GitHub API (unauthenticated: 60 req/h). It is safe for concurrent use.
type Checker struct {
	current  string
	enabled  bool
	endpoint string
	client   *http.Client

	mu     sync.Mutex
	cached *Info
}

// NewChecker returns a checker for the given build version. When enabled is
// false the remote check is skipped and only the local version is reported.
func NewChecker(current string, enabled bool) *Checker {
	return &Checker{
		current:  strings.TrimSpace(current),
		enabled:  enabled,
		endpoint: releasesAPI,
		client:   &http.Client{Timeout: httpTimeout},
	}
}

func (c *Checker) overrideEndpoint(url string) { c.endpoint = url }

// Current returns the running build version.
func (c *Checker) Current() string { return c.current }

// Check returns the current update status. It serves a cached result within the
// TTL; force bypasses the TTL but still honors a short floor so repeated clicks
// cannot spam the upstream API. Dev builds and a disabled checker never hit the
// network.
func (c *Checker) Check(ctx context.Context, force bool) Info {
	base := Info{Current: c.current, Dev: IsDev(c.current)}
	if base.Dev {
		return base
	}
	if !c.enabled {
		base.CheckDisabled = true
		return base
	}

	c.mu.Lock()
	if c.cached != nil && c.cached.CheckedAt != nil {
		age := time.Since(*c.cached.CheckedAt)
		if age < checkTTL || (force && age < refreshFloor) {
			info := *c.cached
			c.mu.Unlock()
			return info
		}
	}
	c.mu.Unlock()

	info := c.fetch(ctx, base)

	c.mu.Lock()
	c.cached = &info
	c.mu.Unlock()
	return info
}

func (c *Checker) fetch(ctx context.Context, base Info) Info {
	now := time.Now().UTC()
	base.CheckedAt = &now
	base.ReleaseURL = releasesPage

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		base.Error = "update check failed"
		return base
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "shellcn/"+c.current)

	resp, err := c.client.Do(req)
	if err != nil {
		base.Error = "could not reach the update server"
		return base
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		base.Error = "could not reach the update server"
		return base
	}

	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		base.Error = "update check failed"
		return base
	}

	latest := strings.TrimSpace(payload.TagName)
	if !semver.IsValid(latest) {
		return base
	}
	base.Latest = latest
	if payload.HTMLURL != "" {
		base.ReleaseURL = payload.HTMLURL
	}
	base.UpdateAvailable = semver.Compare(latest, c.current) > 0
	return base
}

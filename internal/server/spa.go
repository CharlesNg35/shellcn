package server

import (
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin/webproxy"
)

// spaHandler serves the embedded SPA: a real asset is returned as-is; any other
// path falls back to index.html so client-side routing works on deep links.
func (s *Server) spaHandler() http.HandlerFunc {
	fileServer := http.FileServerFS(s.deps.StaticFS)
	return func(w http.ResponseWriter, r *http.Request) {
		if target := webProxyEscapeRedirectTarget(r); target != "" {
			http.Redirect(w, r, target, http.StatusTemporaryRedirect)
			return
		}
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			clean = "index.html"
		}
		info, err := fs.Stat(s.deps.StaticFS, clean)
		if err != nil || info.IsDir() {
			// Not a real file → serve the SPA shell.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}

func webProxyEscapeRedirectTarget(r *http.Request) string {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return ""
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return ""
	}
	cookie, err := r.Cookie(webproxy.PrefixCookieName)
	if err != nil {
		return ""
	}
	prefixes := webProxyPrefixesFromCookie(cookie.Value)
	if len(prefixes) == 0 {
		return ""
	}
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Path == "" {
		return ""
	}
	if ref.Host != "" && !strings.EqualFold(ref.Host, r.Host) {
		return ""
	}
	prefix := matchingWebProxyPrefix(prefixes, ref.Path)
	if prefix == "" {
		return ""
	}
	escaped := r.URL.EscapedPath()
	if escaped == "" {
		escaped = "/"
	}
	return prefix + escaped + querySuffix(r.URL.RawQuery)
}

func webProxyPrefixesFromCookie(value string) []string {
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return nil
	}
	parts := strings.Split(decoded, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		prefix := strings.TrimSpace(part)
		if validWebProxyPrefix(prefix) {
			out = append(out, prefix)
		}
	}
	return out
}

func matchingWebProxyPrefix(prefixes []string, refPath string) string {
	var best string
	for _, prefix := range prefixes {
		if refPath != prefix && !strings.HasPrefix(refPath, prefix+"/") {
			continue
		}
		if len(prefix) > len(best) {
			best = prefix
		}
	}
	return best
}

func validWebProxyPrefix(prefix string) bool {
	if !strings.HasPrefix(prefix, "/api/connections/") || !strings.Contains(prefix, "/proxy/") {
		return false
	}
	u, err := url.Parse(prefix)
	return err == nil && !u.IsAbs() && u.Host == "" && u.RawQuery == "" && u.Fragment == ""
}

func querySuffix(raw string) string {
	if raw == "" {
		return ""
	}
	return "?" + raw
}

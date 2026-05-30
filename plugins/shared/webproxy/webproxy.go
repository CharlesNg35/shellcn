// Package webproxy reverse-proxies a browser to an upstream web app and rewrites
// the response so the app works under a gateway sub-path. It is protocol-neutral:
// a plugin resolves how to reach the upstream (a Kubernetes Service/Pod via the
// API server proxy, a Docker container's port, …) and hands the request here for
// the proxying + HTML/redirect/cookie rewriting and the in-scope service worker.
package webproxy

import (
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// SWFile is the in-scope service worker that re-routes the app's root-absolute
// requests (bundler chunks, dynamic imports, CSS) under the proxy prefix.
const SWFile = "__shellcn_sw.js"

// IsTLSPort is a best-effort guess that a port serves TLS, by the conventional
// "443" suffix (443, 8443, 9443, …). Used only when no protocol metadata exists.
func IsTLSPort(port int) bool {
	return strings.Contains(strconv.Itoa(port), "443")
}

// WebSchemeFromName reads "http"/"https" from a port's conventional name (a
// Kubernetes/Swarm port name), reporting ok=false when it names no web protocol.
func WebSchemeFromName(name string) (scheme string, ok bool) {
	switch n := strings.ToLower(name); {
	case strings.Contains(n, "https"):
		return "https", true
	case n == "http" || n == "web" || strings.HasPrefix(n, "http"):
		return "http", true
	default:
		return "", false
	}
}

// Options describes one proxied request.
type Options struct {
	// Base is the upstream's scheme://host; Transport dials it.
	Base      *url.URL
	Transport http.RoundTripper
	// UpstreamPath/UpstreamRawPath is the path to request on the upstream.
	UpstreamPath    string
	UpstreamRawPath string
	// PublicPrefix is the gateway path the app is served under (for rewriting).
	PublicPrefix string
	// SourcePrefix is a prefix the upstream itself injects into the app's links
	// (e.g. the Kubernetes API server proxy path) and that must be mapped back to
	// PublicPrefix. Empty when proxying straight to the app (Docker), where the
	// app emits its own root-relative paths.
	SourcePrefix string
}

// Serve reverse-proxies r to the upstream and rewrites the response so the app's
// absolute paths, redirects, fetches, and assets resolve back under PublicPrefix.
func Serve(w http.ResponseWriter, r *http.Request, o Options) {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = o.Base.Scheme
			req.URL.Host = o.Base.Host
			req.URL.Path = o.UpstreamPath
			req.URL.RawPath = o.UpstreamRawPath
			req.Host = o.Base.Host
			req.Header.Set("Accept-Encoding", "identity")
		},
		Transport:     o.Transport,
		FlushInterval: -1,
		//nolint:bodyclose // body is read+closed in the HTML branch; otherwise the ReverseProxy owns it.
		ModifyResponse: rewriteResponse(o.SourcePrefix, o.PublicPrefix),
	}
	proxy.ServeHTTP(w, r)
}

var rootRelAttr = regexp.MustCompile(`(\s(?:href|src|action)=")(/[^"/][^"]*)"`)

// rewriteResponse adjusts an upstream response so it works under prefix. When the
// upstream injects its own sourcePrefix (the API server proxy), root-relative URLs
// are mapped from it back to prefix; otherwise (a direct upstream) bare
// root-relative URLs are prefixed. It also rewrites redirect Location + Set-Cookie
// paths, drops framing/CSP, and for HTML injects a <base> + fetch/XHR shim.
func rewriteResponse(sourcePrefix, prefix string) func(*http.Response) error {
	return func(resp *http.Response) error {
		switch loc := resp.Header.Get("Location"); {
		case sourcePrefix != "" && strings.HasPrefix(loc, sourcePrefix):
			resp.Header.Set("Location", prefix+strings.TrimPrefix(loc, sourcePrefix))
		case strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//") && !strings.HasPrefix(loc, prefix):
			resp.Header.Set("Location", prefix+loc)
		}
		rewriteCookiePaths(resp.Header, prefix)
		// Allow embedding + inline shim by relaxing framing/CSP.
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("X-Frame-Options")

		if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			return nil
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		html := string(body)
		// Map the upstream's injected prefix back to ours (when present), then
		// prefix any root-relative URLs left bare (skipping ones already prefixed),
		// and inject our base+shim so they aren't re-prefixed.
		if sourcePrefix != "" {
			html = strings.ReplaceAll(html, sourcePrefix, prefix)
		}
		html = rootRelAttr.ReplaceAllStringFunc(html, func(m string) string {
			g := rootRelAttr.FindStringSubmatch(m)
			if strings.HasPrefix(g[2], prefix) {
				return m
			}
			return g[1] + prefix + g[2] + `"`
		})
		// The PWA manifest is fetched without credentials by default, which the
		// authenticated proxy rejects; ask the browser to send them.
		html = strings.ReplaceAll(html, `rel="manifest"`, `rel="manifest" crossorigin="use-credentials"`)
		html = injectShim(html, prefix)
		resp.Body = io.NopCloser(strings.NewReader(html))
		resp.ContentLength = int64(len(html))
		resp.Header.Set("Content-Length", strconv.Itoa(len(html)))
		return nil
	}
}

func rewriteCookiePaths(h http.Header, prefix string) {
	cookies := h.Values("Set-Cookie")
	if len(cookies) == 0 {
		return
	}
	h.Del("Set-Cookie")
	for _, c := range cookies {
		h.Add("Set-Cookie", rewriteCookiePath(c, prefix))
	}
}

func rewriteCookiePath(cookie, prefix string) string {
	parts := strings.Split(cookie, ";")
	for i, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if strings.EqualFold(kv[0], "Path") {
			parts[i] = " Path=" + prefix
		}
	}
	return strings.Join(parts, ";")
}

// injectShim adds a <base> and a shim after <head> so the app's requests stay
// under the proxy prefix. It rewrites root-absolute URLs in fetch/XHR and, crucially,
// in runtime-injected assets — script/link/img src/href and setAttribute — so
// bundler-loaded chunks and styles (which bypass fetch) resolve under the prefix.
func injectShim(html, prefix string) string {
	shim := `<base href="` + prefix + `/"><script>(function(){var p=` + jsString(prefix) + `;
function fix(u){return (typeof u==="string"&&u.charAt(0)==="/"&&u.charAt(1)!=="/"&&u.indexOf(p)!==0)?p+u:u;}
var of=window.fetch;if(of){window.fetch=function(i,o){try{if(typeof i==="string")i=fix(i);else if(i&&i.url)i=new Request(fix(i.url),i);}catch(e){}return of.call(this,i,o);};}
var ox=XMLHttpRequest.prototype.open;XMLHttpRequest.prototype.open=function(m,u){return ox.apply(this,[m,fix(u)].concat([].slice.call(arguments,2)));};
function patch(proto,prop){var d=Object.getOwnPropertyDescriptor(proto,prop);if(d&&d.set)Object.defineProperty(proto,prop,{configurable:true,enumerable:d.enumerable,get:function(){return d.get.call(this);},set:function(v){d.set.call(this,fix(v));}});}
try{patch(HTMLScriptElement.prototype,"src");patch(HTMLLinkElement.prototype,"href");patch(HTMLImageElement.prototype,"src");}catch(e){}
var sa=Element.prototype.setAttribute;Element.prototype.setAttribute=function(n,v){return sa.call(this,n,(n==="src"||n==="href")&&typeof v==="string"?fix(v):v);};
["pushState","replaceState"].forEach(function(m){var o=history[m];if(o)history[m]=function(s,t,u){return o.call(this,s,t,typeof u==="string"?fix(u):u);};});
if(navigator.serviceWorker){try{navigator.serviceWorker.register(p+"/` + SWFile + `").then(function(){if(!navigator.serviceWorker.controller){var k="scnsw:"+p;if(!sessionStorage.getItem(k)){sessionStorage.setItem(k,"1");navigator.serviceWorker.ready.then(function(){location.reload();});}}});}catch(e){}}
})();</script>`
	if i := headInsertIndex(html); i >= 0 {
		return html[:i] + shim + html[i:]
	}
	return shim + html
}

// headInsertIndex returns the index just after the opening <head ...> tag.
func headInsertIndex(html string) int {
	lower := strings.ToLower(html)
	i := strings.Index(lower, "<head")
	if i < 0 {
		return -1
	}
	if end := strings.IndexByte(lower[i:], '>'); end >= 0 {
		return i + end + 1
	}
	return -1
}

func jsString(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

// ServeWorker returns the service worker. Served from under the prefix, its
// default scope is the proxy path, so it controls the app's page and rewrites any
// root-absolute request it makes back under the prefix.
func ServeWorker(w http.ResponseWriter, prefix string) {
	w.Header().Set("Content-Type", "text/javascript")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Service-Worker-Allowed", prefix+"/")
	_, _ = io.WriteString(w, `var P=`+jsString(prefix)+`;
self.addEventListener("install",function(){self.skipWaiting();});
self.addEventListener("activate",function(e){e.waitUntil(self.clients.claim());});
self.addEventListener("fetch",function(e){var u;try{u=new URL(e.request.url);}catch(_){return;}
if(u.origin===self.location.origin&&u.pathname.charAt(0)==="/"&&u.pathname.indexOf(P+"/")!==0){
u.pathname=P+u.pathname;
e.respondWith(fetch(u.href,{method:e.request.method,headers:e.request.headers,credentials:"include"}));}});`)
}

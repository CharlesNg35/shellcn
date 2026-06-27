// Package webproxy reverse-proxies a browser to an upstream web app and rewrites
// the response so the app works under a gateway sub-path. A plugin resolves how
// to reach the upstream and hands the request here for proxying, response
// rewriting, and the in-scope service worker.
package webproxy

import (
	"io"
	"net"
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
	// and that must be mapped back to PublicPrefix. Empty when proxying straight
	// to an app that emits its own root-relative paths.
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
			req.Header.Set("X-Forwarded-Host", r.Host)
			req.Header.Set("X-Forwarded-Prefix", o.PublicPrefix)
			req.Header.Set("X-Forwarded-Proto", forwardedProto(r))
			req.Header.Set("X-Forwarded-Uri", r.URL.RequestURI())
			req.Header.Set("Forwarded", forwardedHeader(r))
		},
		Transport:     o.Transport,
		FlushInterval: -1,
		//nolint:bodyclose // body is read+closed in the HTML branch; otherwise the ReverseProxy owns it.
		ModifyResponse: rewriteResponse(o.Base, o.SourcePrefix, o.PublicPrefix, r.Host),
	}
	proxy.ServeHTTP(w, r)
}

// URL-bearing HTML attributes whose root-relative value (incl. a bare "/") needs
// prefixing; protocol-relative "//host" is excluded.
var (
	rootRelAttr       = regexp.MustCompile(`(\s(?:href|src|action|formaction|poster)=")(/(?:[^"/][^"]*)?)"`)
	rootRelSingleAttr = regexp.MustCompile(`(\s(?:href|src|action|formaction|poster)=')(/(?:[^'/][^']*)?)'`)
	srcsetAttr        = regexp.MustCompile(`(\ssrcset=")([^"]*)"`)
	srcsetSingleAttr  = regexp.MustCompile(`(\ssrcset=')([^']*)'`)
	metaRefresh       = regexp.MustCompile(`(?i)(content="\s*\d+\s*;\s*url=)(/[^"/][^"]*)"`)
	metaRefreshSingle = regexp.MustCompile(`(?i)(content='\s*\d+\s*;\s*url=)(/[^'/][^']*)'`)
	metaCSP           = regexp.MustCompile(`(?i)<meta[^>]+http-equiv="content-security-policy"[^>]*>`)
	cssURL            = regexp.MustCompile(`url\(\s*(['"]?)(/[^'")\s]*)(['"]?)\s*\)`)
)

// rewriteResponse maps an upstream response back under prefix: Location, Set-Cookie,
// framing/CSP headers, and HTML/CSS bodies.
func rewriteResponse(base *url.URL, sourcePrefix, prefix, publicHost string) func(*http.Response) error {
	upstreamOrigin, upstreamHost := "", ""
	if base != nil {
		upstreamOrigin = base.Scheme + "://" + base.Host
		upstreamHost = base.Host
	}
	return func(resp *http.Response) error {
		if loc := resp.Header.Get("Location"); loc != "" {
			resp.Header.Set("Location", mapLocation(loc, prefix, publicHost, upstreamHost, sourcePrefix))
		}
		rewriteCookiePaths(resp.Header, prefix)
		// Allow embedding + the inline shim by relaxing framing/CSP.
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("Content-Security-Policy-Report-Only")
		resp.Header.Del("X-Frame-Options")

		ct := resp.Header.Get("Content-Type")
		isHTML := strings.Contains(ct, "text/html")
		isCSS := strings.Contains(ct, "text/css")
		if !isHTML && !isCSS {
			return nil
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		out := string(body)
		if publicHost != "" {
			out = stripOrigins(out, prefix, "https://"+publicHost, "http://"+publicHost)
		}
		if isHTML {
			out = rewriteHTML(out, sourcePrefix, upstreamOrigin, prefix)
		} else {
			out = rewriteCSS(out, upstreamOrigin, prefix)
		}
		resp.Body = io.NopCloser(strings.NewReader(out))
		resp.ContentLength = int64(len(out))
		resp.Header.Set("Content-Length", strconv.Itoa(len(out)))
		return nil
	}
}

// mapLocation rewrites a redirect Location to stay under prefix. It maps targets on
// the app itself (root-relative, upstream/public host, or protocol-relative) and
// leaves external redirects untouched; idempotent if already prefixed.
func mapLocation(loc, prefix, publicHost, upstreamHost, sourcePrefix string) string {
	if loc == "" {
		return loc
	}
	if sourcePrefix != "" && strings.HasPrefix(loc, sourcePrefix) {
		return prefix + strings.TrimPrefix(loc, sourcePrefix)
	}
	u, err := url.Parse(loc)
	if err != nil {
		return loc
	}
	if u.Host != "" {
		if !sameHost(u.Host, publicHost) && !sameHost(u.Host, upstreamHost) {
			return loc
		}
		u.Scheme, u.Opaque, u.Host, u.User = "", "", "", nil
	} else if u.Path == "" || u.Path[0] != '/' {
		return loc
	}
	if u.Path == prefix || strings.HasPrefix(u.Path, prefix+"/") {
		return u.String()
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Path = prefix + u.Path
	u.RawPath = ""
	return u.String()
}

func sameHost(a, b string) bool { return b != "" && strings.EqualFold(a, b) }

// stripOrigins maps each origin to prefix, skipping occurrences already followed by
// prefix so it never double-prefixes.
func stripOrigins(s, prefix string, origins ...string) string {
	for _, origin := range origins {
		if origin == "" || !strings.Contains(s, origin) {
			continue
		}
		var b strings.Builder
		rest := s
		for {
			i := strings.Index(rest, origin)
			if i < 0 {
				b.WriteString(rest)
				break
			}
			b.WriteString(rest[:i])
			rest = rest[i+len(origin):]
			if !strings.HasPrefix(rest, prefix) {
				b.WriteString(prefix)
			}
		}
		s = b.String()
	}
	return s
}

// rewriteHTML prefixes the app's URLs and injects the runtime shim. No <base> is
// added: relative URLs resolve naturally against the current path, so forms and
// fragments behave as un-proxied.
func rewriteHTML(html, sourcePrefix, upstreamOrigin, prefix string) string {
	if sourcePrefix != "" {
		html = strings.ReplaceAll(html, sourcePrefix, prefix)
	}
	if upstreamOrigin != "" {
		html = strings.ReplaceAll(html, upstreamOrigin, prefix)
	}
	html = metaCSP.ReplaceAllString(html, "")
	html = rootRelAttr.ReplaceAllStringFunc(html, func(m string) string {
		g := rootRelAttr.FindStringSubmatch(m)
		return g[1] + prefixRootRel(g[2], prefix) + `"`
	})
	html = rootRelSingleAttr.ReplaceAllStringFunc(html, func(m string) string {
		g := rootRelSingleAttr.FindStringSubmatch(m)
		return g[1] + prefixRootRel(g[2], prefix) + `'`
	})
	html = srcsetAttr.ReplaceAllStringFunc(html, func(m string) string {
		g := srcsetAttr.FindStringSubmatch(m)
		return g[1] + rewriteSrcset(g[2], prefix) + `"`
	})
	html = srcsetSingleAttr.ReplaceAllStringFunc(html, func(m string) string {
		g := srcsetSingleAttr.FindStringSubmatch(m)
		return g[1] + rewriteSrcset(g[2], prefix) + `'`
	})
	html = metaRefresh.ReplaceAllStringFunc(html, func(m string) string {
		g := metaRefresh.FindStringSubmatch(m)
		return g[1] + prefixRootRel(g[2], prefix) + `"`
	})
	html = metaRefreshSingle.ReplaceAllStringFunc(html, func(m string) string {
		g := metaRefreshSingle.FindStringSubmatch(m)
		return g[1] + prefixRootRel(g[2], prefix) + `'`
	})
	html = rewriteInlineCSS(html, prefix)
	// The PWA manifest is fetched without credentials by default, which the
	// authenticated proxy rejects; ask the browser to send them.
	html = strings.ReplaceAll(html, `rel="manifest"`, `rel="manifest" crossorigin="use-credentials"`)
	return injectShim(html, prefix)
}

// prefixRootRel prefixes a not-yet-prefixed root-relative URL (incl. bare "/").
func prefixRootRel(u, prefix string) string {
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") && !strings.HasPrefix(u, prefix) {
		return prefix + u
	}
	return u
}

// rewriteSrcset prefixes each candidate URL in a srcset value (`url 1x, url 2x`).
func rewriteSrcset(srcset, prefix string) string {
	parts := strings.Split(srcset, ",")
	for i, part := range parts {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		fields[0] = prefixRootRel(fields[0], prefix)
		parts[i] = strings.Join(fields, " ")
	}
	return strings.Join(parts, ", ")
}

var styleBlock = regexp.MustCompile(`(?is)(<style[^>]*>)(.*?)(</style>)`)

// rewriteInlineCSS prefixes url() targets inside <style> blocks.
func rewriteInlineCSS(html, prefix string) string {
	return styleBlock.ReplaceAllStringFunc(html, func(m string) string {
		g := styleBlock.FindStringSubmatch(m)
		return g[1] + rewriteCSS(g[2], "", prefix) + g[3]
	})
}

// rewriteCSS prefixes root-relative url() targets in a stylesheet and maps any
// absolute upstream-origin ones back under prefix.
func rewriteCSS(css, upstreamOrigin, prefix string) string {
	if upstreamOrigin != "" {
		css = strings.ReplaceAll(css, upstreamOrigin, prefix)
	}
	return cssURL.ReplaceAllStringFunc(css, func(m string) string {
		g := cssURL.FindStringSubmatch(m)
		return "url(" + g[1] + prefixRootRel(g[2], prefix) + g[3] + ")"
	})
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

// rewriteCookiePath scopes a cookie to the prefix and drops any upstream Domain. A
// __Host- cookie is left as-is: its spec requires Path=/, so narrowing it would void
// the cookie.
func rewriteCookiePath(cookie, prefix string) string {
	if name := strings.TrimSpace(strings.SplitN(cookie, "=", 2)[0]); strings.HasPrefix(name, "__Host-") {
		return cookie
	}
	parts := strings.Split(cookie, ";")
	kept := parts[:0]
	hasPath := false
	for _, p := range parts {
		key, _, _ := strings.Cut(strings.TrimSpace(p), "=")
		switch {
		case strings.EqualFold(key, "Domain"):
			// An upstream Domain is rejected by the browser on the gateway host.
			continue
		case strings.EqualFold(key, "Path"):
			kept = append(kept, " Path="+prefix)
			hasPath = true
		default:
			kept = append(kept, p)
		}
	}
	if !hasPath {
		kept = append(kept, " Path="+prefix)
	}
	return strings.Join(kept, ";")
}

// injectShim inserts a script that keeps the app's runtime requests and navigations
// under the prefix — fetch/XHR, WebSocket/EventSource/Worker, history, location
// (assign/replace/href), assets, and form actions — plus the in-scope service
// worker.
func injectShim(html, prefix string) string {
	shim := `<script>(function(){var p=` + jsString(prefix) + `;
function fix(u){if(typeof u!=="string")return u;if(u.charAt(0)==="/"&&u.charAt(1)!=="/"&&u.indexOf(p)!==0)return p+u;var o=location.origin;if(u.indexOf(o+"/")===0&&u.slice(o.length).indexOf(p)!==0)return o+p+u.slice(o.length);var wo=(location.protocol==="https:"?"wss://":"ws://")+location.host;if(u.indexOf(wo+"/")===0&&u.slice(wo.length).indexOf(p)!==0)return wo+p+u.slice(wo.length);return u;}
var of=window.fetch;if(of){window.fetch=function(i,o){try{if(typeof i==="string")i=fix(i);else if(i&&i.url)i=new Request(fix(i.url),i);}catch(e){}return of.call(this,i,o);};}
var ox=XMLHttpRequest.prototype.open;XMLHttpRequest.prototype.open=function(m,u){return ox.apply(this,[m,fix(u)].concat([].slice.call(arguments,2)));};
function wrap(C){if(!C)return C;function W(){var a=[].slice.call(arguments);if(a.length)a[0]=fix(a[0]);return new (Function.prototype.bind.apply(C,[null].concat(a)))();}W.prototype=C.prototype;["CONNECTING","OPEN","CLOSING","CLOSED"].forEach(function(k){if(k in C)W[k]=C[k];});return W;}
try{window.WebSocket=wrap(window.WebSocket);window.EventSource=wrap(window.EventSource);window.Worker=wrap(window.Worker);}catch(e){}
function patch(proto,prop){var d=Object.getOwnPropertyDescriptor(proto,prop);if(d&&d.set)Object.defineProperty(proto,prop,{configurable:true,enumerable:d.enumerable,get:function(){return d.get.call(this);},set:function(v){d.set.call(this,fix(v));}});}
try{patch(HTMLScriptElement.prototype,"src");patch(HTMLLinkElement.prototype,"href");patch(HTMLImageElement.prototype,"src");patch(HTMLAnchorElement.prototype,"href");}catch(e){}
var sa=Element.prototype.setAttribute;Element.prototype.setAttribute=function(n,v){return sa.call(this,n,(typeof v==="string"&&/^(src|href|action|formaction|poster)$/i.test(n))?fix(v):v);};
["pushState","replaceState"].forEach(function(m){var o=history[m];if(o)history[m]=function(s,t,u){return o.call(this,s,t,typeof u==="string"?fix(u):u);};});
["assign","replace"].forEach(function(m){var o=location[m];if(o)try{location[m]=function(u){return o.call(location,fix(u));};}catch(e){}});
try{var lh=Object.getOwnPropertyDescriptor(Location.prototype,"href");if(lh&&lh.set)Object.defineProperty(location,"href",{configurable:true,get:function(){return lh.get.call(location);},set:function(v){lh.set.call(location,fix(v));}});}catch(e){}
document.addEventListener("submit",function(e){var f=e.target;if(!f||f.tagName!=="FORM")return;var raw=f.getAttribute("action");if(!raw)return;var g=fix(raw);if(g!==raw)f.setAttribute("action",g);},true);
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

func jsString(s string) string { return strconv.Quote(s) }

func forwardedProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		return strings.Split(proto, ",")[0]
	}
	return "http"
}

func forwardedHeader(r *http.Request) string {
	parts := []string{"host=" + quoteForwardedValue(r.Host), "proto=" + quoteForwardedValue(forwardedProto(r))}
	if ip := clientIP(r); ip != "" {
		parts = append(parts, "for="+quoteForwardedValue(ip))
	}
	return strings.Join(parts, ";")
}

func clientIP(r *http.Request) string {
	if prior := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); prior != "" {
		return strings.TrimSpace(strings.Split(prior, ",")[0])
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func quoteForwardedValue(v string) string {
	return strconv.Quote(strings.ReplaceAll(v, `"`, ""))
}

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
e.respondWith((async function(){var r=e.request;var init={method:r.method,headers:r.headers,credentials:"include"};if(r.method!=="GET"&&r.method!=="HEAD"){init.body=await r.blob();}return fetch(u.href,init);})());}});`)
}

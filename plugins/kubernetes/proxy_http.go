package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// ServeHTTPProxy reverse-proxies a browser request to an in-cluster Service or
// Pod port via the API server's proxy subresource, using the session's REST
// transport (so it works over both transports). The incoming path is
// `/{services|pods}/{ns}/{name}/{port}/{rest...}`. Responses are rewritten so the
// app's own absolute paths, redirects, and fetches resolve back under the proxy.
func (s *Session) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	apiPath, prefix, ok := s.proxyPaths(r.URL.Path)
	if !ok {
		http.Error(w, "unsupported proxy target", http.StatusBadRequest)
		return
	}
	base, err := url.Parse(s.rest.Host)
	if err != nil || base.Host == "" {
		http.Error(w, "bad upstream", http.StatusBadGateway)
		return
	}
	rt, err := rest.TransportFor(s.rest)
	if err != nil {
		http.Error(w, "transport: "+err.Error(), http.StatusBadGateway)
		return
	}
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = base.Scheme
			req.URL.Host = base.Host
			req.URL.Path = apiPath
			req.Host = base.Host
			// Identity so we can rewrite HTML bodies without gunzipping.
			req.Header.Set("Accept-Encoding", "identity")
		},
		Transport:     rt,
		FlushInterval: -1,
		//nolint:bodyclose // body is read+closed in the HTML branch; otherwise the ReverseProxy owns it.
		ModifyResponse: rewriteProxyResponse(prefix),
	}
	proxy.ServeHTTP(w, r)
}

// proxyPaths maps the public sub-path to the API server proxy path and the
// public prefix the app is served under (for response rewriting). The port
// segment may carry an "https:" marker to proxy over TLS to the target.
func (s *Session) proxyPaths(sub string) (apiPath, prefix string, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(sub, "/"), "/", 5)
	if len(parts) < 4 {
		return "", "", false
	}
	kind, ns, name, portSeg := parts[0], parts[1], parts[2], parts[3]
	if kind != "services" && kind != "pods" {
		return "", "", false
	}
	rest := ""
	if len(parts) == 5 {
		rest = parts[4]
	}
	// "https:8443" → the API server proxies over TLS; otherwise plain http.
	target := name + ":" + portSeg
	if realPort, https := strings.CutPrefix(portSeg, "https:"); https {
		target = "https:" + name + ":" + realPort
	}
	apiPath = fmt.Sprintf("/api/v1/namespaces/%s/%s/%s/proxy/%s", ns, kind, target, rest)
	prefix = fmt.Sprintf("/api/connections/%s/proxy/%s/%s/%s/%s", s.connID, kind, ns, name, portSeg)
	return apiPath, prefix, true
}

var rootRelAttr = regexp.MustCompile(`(\s(?:href|src|action)=")(/[^"/][^"]*)"`)

// rewriteProxyResponse adjusts an upstream response so it works under prefix:
// rewrites redirect Location + Set-Cookie paths, drops framing/CSP restrictions,
// and (for HTML) injects a <base> + a small fetch/XHR shim and rewrites
// root-relative asset URLs.
func rewriteProxyResponse(prefix string) func(*http.Response) error {
	return func(resp *http.Response) error {
		if loc := resp.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//") && !strings.HasPrefix(loc, prefix) {
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
		// Rewrite the app's own root-relative URLs first, then inject our
		// base+shim (so they aren't themselves re-prefixed).
		html := rootRelAttr.ReplaceAllString(string(body), "${1}"+prefix+"$2\"")
		html = injectProxyShim(html, prefix)
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

// injectProxyShim adds a <base> and a fetch/XHR/path shim after <head> so the
// app's relative and root-absolute requests stay under the proxy prefix.
func injectProxyShim(html, prefix string) string {
	shim := `<base href="` + prefix + `/"><script>(function(){var p=` + jsString(prefix) + `;
function fix(u){return (typeof u==="string"&&u.charAt(0)==="/"&&u.charAt(1)!=="/"&&u.indexOf(p)!==0)?p+u:u;}
var of=window.fetch;if(of){window.fetch=function(i,o){try{if(typeof i==="string")i=fix(i);else if(i&&i.url)i=new Request(fix(i.url),i);}catch(e){}return of.call(this,i,o);};}
var ox=XMLHttpRequest.prototype.open;XMLHttpRequest.prototype.open=function(m,u){return ox.apply(this,[m,fix(u)].concat([].slice.call(arguments,2)));};
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

// ServiceProxyURL returns the gateway URL that proxies to a Service's web port
// for an "Open in browser" link. It picks the most likely web port (an http
// port, else an https one, else the first) unless an explicit port is given.
func ServiceProxyURL(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	ns, name := rc.Param("namespace"), rc.Param("name")
	portSeg := rc.Param("port")
	if portSeg == "" {
		svc, err := s.clientset.CoreV1().Services(ns).Get(rc.Ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, apiErr(err)
		}
		portSeg, err = pickServicePort(svc.Spec.Ports)
		if err != nil {
			return nil, err
		}
	}
	u := fmt.Sprintf("/api/connections/%s/proxy/services/%s/%s/%s/", url.PathEscape(s.connID), url.PathEscape(ns), url.PathEscape(name), portSeg)
	return map[string]any{"url": u}, nil
}

// pickServicePort returns the port segment to proxy ("8080" for http,
// "https:8443" for a TLS port). It prefers a port that declares a web protocol
// (via appProtocol or its name), then falls back to the first port. The scheme
// is taken from appProtocol/name — not guessed from the port number.
func pickServicePort(ports []corev1.ServicePort) (string, error) {
	if len(ports) == 0 {
		return "", fmt.Errorf("%w: service exposes no ports", plugin.ErrInvalidInput)
	}
	for _, p := range ports {
		if scheme, ok := webScheme(p); ok {
			return portSegment(p.Port, scheme), nil
		}
	}
	first := ports[0]
	scheme, _ := webScheme(first)
	if scheme == "" {
		scheme = "http"
	}
	return portSegment(first.Port, scheme), nil
}

func portSegment(port int32, scheme string) string {
	seg := strconv.Itoa(int(port))
	if scheme == "https" {
		return "https:" + seg
	}
	return seg
}

// webScheme returns "http"/"https" when a port declares a web protocol via its
// (GA) appProtocol or by the conventional port name, and ok=false otherwise.
func webScheme(p corev1.ServicePort) (string, bool) {
	if p.AppProtocol != nil {
		switch strings.ToLower(*p.AppProtocol) {
		case "https":
			return "https", true
		case "http", "http2":
			return "http", true
		default:
			return "", false
		}
	}
	switch n := strings.ToLower(p.Name); {
	case strings.Contains(n, "https"):
		return "https", true
	case n == "http" || n == "web" || strings.HasPrefix(n, "http"):
		return "http", true
	default:
		return "", false
	}
}

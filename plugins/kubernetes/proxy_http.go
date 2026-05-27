package kubernetes

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/charlesng/shellcn/internal/plugin"
)

// ServeHTTPProxy reverse-proxies a browser request to an in-cluster Service or
// Pod port via the API server's proxy subresource, using the session's REST
// transport — so it works over both direct and agent transport. The incoming
// path is `/{services|pods}/{ns}/{name}/{port}/{rest...}`.
func (s *Session) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	apiPath, ok := apiProxyPath(r.URL.Path)
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
		},
		Transport:     rt,
		FlushInterval: -1,
	}
	proxy.ServeHTTP(w, r)
}

func apiProxyPath(sub string) (string, bool) {
	parts := strings.SplitN(strings.TrimPrefix(sub, "/"), "/", 5)
	if len(parts) < 4 {
		return "", false
	}
	kind, ns, name, port := parts[0], parts[1], parts[2], parts[3]
	rest := ""
	if len(parts) == 5 {
		rest = parts[4]
	}
	switch kind {
	case "services":
		return fmt.Sprintf("/api/v1/namespaces/%s/services/%s:%s/proxy/%s", ns, name, port, rest), true
	case "pods":
		return fmt.Sprintf("/api/v1/namespaces/%s/pods/%s:%s/proxy/%s", ns, name, port, rest), true
	default:
		return "", false
	}
}

// ServiceProxyURL returns the gateway URL that proxies to a Service's web port,
// for an "Open in browser" link (Rancher-style). Defaults to the first port.
func ServiceProxyURL(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	ns, name := rc.Param("namespace"), rc.Param("name")
	port := rc.Param("port")
	if port == "" {
		svc, err := s.clientset.CoreV1().Services(ns).Get(rc.Ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, apiErr(err)
		}
		if len(svc.Spec.Ports) == 0 {
			return nil, fmt.Errorf("%w: service has no ports", plugin.ErrInvalidInput)
		}
		port = strconv.Itoa(int(svc.Spec.Ports[0].Port))
	}
	u := fmt.Sprintf("/api/connections/%s/proxy/services/%s/%s/%s/", url.PathEscape(s.connID), url.PathEscape(ns), url.PathEscape(name), url.PathEscape(port))
	return map[string]any{"url": u}, nil
}

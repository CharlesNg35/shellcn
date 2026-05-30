package kubernetes

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/webproxy"
)

// ServeHTTPProxy reverse-proxies a browser request to an in-cluster Service or
// Pod port via the API server's proxy subresource, using the session's REST
// transport (so it works over both transports). The incoming path is
// `/{services|pods}/{ns}/{name}/{port}/{rest...}`. The generic rewriting +
// service worker live in the shared webproxy package; here we only resolve the
// upstream (the API server proxy path) and the prefix it injects.
func (s *Session) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	apiPath, apiPrefix, prefix, ok := s.proxyPaths(r.URL.Path)
	if !ok {
		http.Error(w, "unsupported proxy target", http.StatusBadRequest)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/"+webproxy.SWFile) {
		webproxy.ServeWorker(w, prefix)
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
	// Preserve the original path encoding (Next.js route-group chunks carry
	// %5B/%28 etc.) so the upstream receives the exact filename.
	apiRawPath, _, _, _ := s.proxyPaths(r.URL.EscapedPath())
	webproxy.Serve(w, r, webproxy.Options{
		Base:            base,
		Transport:       rt,
		UpstreamPath:    apiPath,
		UpstreamRawPath: apiRawPath,
		PublicPrefix:    prefix,
		SourcePrefix:    apiPrefix,
	})
}

// proxyPaths maps the public sub-path to the API server proxy path, the prefix
// the API server itself prepends when rewriting the app's links (apiPrefix), and
// the public prefix the app is served under (for response rewriting). The port
// segment may carry an "https:" marker to proxy over TLS to the target.
func (s *Session) proxyPaths(sub string) (apiPath, apiPrefix, prefix string, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(sub, "/"), "/", 5)
	if len(parts) < 4 {
		return "", "", "", false
	}
	kind, ns, name, portSeg := parts[0], parts[1], parts[2], parts[3]
	if kind != "services" && kind != "pods" {
		return "", "", "", false
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
	apiPrefix = fmt.Sprintf("/api/v1/namespaces/%s/%s/%s/proxy", ns, kind, target)
	apiPath = apiPrefix + "/" + rest
	prefix = fmt.Sprintf("/api/connections/%s/proxy/%s/%s/%s/%s", s.connID, kind, ns, name, portSeg)
	return apiPath, apiPrefix, prefix, true
}

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
	return proxyURLResult(s.connID, "services", ns, name, portSeg), nil
}

// PodProxyURL is the pod equivalent of ServiceProxyURL: it proxies straight to a
// pod's container port (a browser-usable, HTTP form of `kubectl port-forward`).
func PodProxyURL(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	ns, name := rc.Param("namespace"), rc.Param("name")
	portSeg := rc.Param("port")
	if portSeg == "" {
		pod, err := s.clientset.CoreV1().Pods(ns).Get(rc.Ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, apiErr(err)
		}
		portSeg, err = pickPodPort(pod.Spec.Containers)
		if err != nil {
			return nil, err
		}
	}
	return proxyURLResult(s.connID, "pods", ns, name, portSeg), nil
}

func proxyURLResult(connID, kind, ns, name, portSeg string) map[string]any {
	u := fmt.Sprintf("/api/connections/%s/proxy/%s/%s/%s/%s/", url.PathEscape(connID), kind, url.PathEscape(ns), url.PathEscape(name), portSeg)
	return map[string]any{"url": u}
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
	scheme, ok := webScheme(first)
	if !ok {
		scheme = defaultScheme(int(first.Port))
	}
	return portSegment(first.Port, scheme), nil
}

// defaultScheme is the fallback when no appProtocol/name declares one: TLS by the
// conventional port, else plain HTTP.
func defaultScheme(port int) string {
	if webproxy.IsTLSPort(port) {
		return "https"
	}
	return "http"
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
	return webproxy.WebSchemeFromName(p.Name)
}

// pickPodPort picks a pod's web container port (preferring a web-named TCP port,
// else the first TCP port), mirroring pickServicePort.
func pickPodPort(containers []corev1.Container) (string, error) {
	var first *corev1.ContainerPort
	for i := range containers {
		for j := range containers[i].Ports {
			p := &containers[i].Ports[j]
			if p.Protocol != "" && p.Protocol != corev1.ProtocolTCP {
				continue
			}
			if scheme, ok := webproxy.WebSchemeFromName(p.Name); ok {
				return portSegment(p.ContainerPort, scheme), nil
			}
			if first == nil {
				first = p
			}
		}
	}
	if first == nil {
		return "", fmt.Errorf("%w: pod exposes no TCP ports", plugin.ErrInvalidInput)
	}
	scheme, ok := webproxy.WebSchemeFromName(first.Name)
	if !ok {
		scheme = defaultScheme(int(first.ContainerPort))
	}
	return portSegment(first.ContainerPort, scheme), nil
}

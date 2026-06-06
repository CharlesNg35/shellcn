package kubernetes

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng35/shellcn/plugins/shared/webproxy"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// ServeHTTPProxy reverse-proxies a browser request to a Service or Pod port. It
// reaches the workload through the pod port-forward subresource — an L4 tunnel —
// so the app's own Authorization/cookies pass through to the backend, which the
// API server's HTTP proxy would otherwise strip. The incoming path is
// `/{services|pods}/{ns}/{name}/{port}/{rest...}`; generic HTML/redirect/cookie
// rewriting and the service worker live in the shared webproxy package.
func (s *Session) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	kind, ns, name, portSeg, rest, ok := splitProxyTarget(r.URL.Path)
	if !ok {
		http.Error(w, "unsupported proxy target", http.StatusBadRequest)
		return
	}
	prefix := fmt.Sprintf("%s/%s/%s/%s/%s", plugin.RequestProxyPrefix(r), kind, ns, name, portSeg)
	if rest == webproxy.SWFile {
		webproxy.ServeWorker(w, prefix)
		return
	}
	scheme, port, ok := schemePort(portSeg)
	if !ok {
		http.Error(w, "bad proxy port", http.StatusBadRequest)
		return
	}
	podNS, podName, podPort, err := s.proxyPodTarget(r.Context(), kind, ns, name, port)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	transport, err := s.proxyTransport(podNS, podName, podPort, scheme == "https")
	if err != nil {
		http.Error(w, "proxy transport: "+err.Error(), http.StatusBadGateway)
		return
	}
	// Preserve the original path encoding (route-group chunks carry %5B/%28 etc.)
	// so the upstream receives the exact filename.
	_, _, _, _, rawRest, _ := splitProxyTarget(r.URL.EscapedPath())
	webproxy.Serve(w, r, webproxy.Options{
		Base:            &url.URL{Scheme: scheme, Host: net.JoinHostPort(name, strconv.Itoa(port))},
		Transport:       transport,
		UpstreamPath:    "/" + rest,
		UpstreamRawPath: "/" + rawRest,
		PublicPrefix:    prefix,
	})
}

// splitProxyTarget parses `/{services|pods}/{ns}/{name}/{port}/{rest...}`.
func splitProxyTarget(sub string) (kind, ns, name, portSeg, rest string, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(sub, "/"), "/", 5)
	if len(parts) < 4 || (parts[0] != "services" && parts[0] != "pods") {
		return "", "", "", "", "", false
	}
	if len(parts) == 5 {
		rest = parts[4]
	}
	return parts[0], parts[1], parts[2], parts[3], rest, true
}

// schemePort splits a port segment ("8080" or "https:8443") into scheme + port.
func schemePort(portSeg string) (scheme string, port int, ok bool) {
	seg := portSeg
	scheme = "http"
	if p, https := strings.CutPrefix(portSeg, "https:"); https {
		seg, scheme = p, "https"
	}
	n, err := strconv.Atoi(seg)
	if err != nil || n <= 0 || n > 65535 {
		return "", 0, false
	}
	return scheme, n, true
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
	return proxyURLResult(rc, "services", ns, name, portSeg), nil
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
	return proxyURLResult(rc, "pods", ns, name, portSeg), nil
}

func proxyURLResult(rc *plugin.RequestContext, kind, ns, name, portSeg string) map[string]any {
	return map[string]any{"url": rc.ProxyURL(kind, ns, name, portSeg)}
}

func openPortSchema(optionsRouteID string) *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{
		Name: "Open",
		Fields: []plugin.Field{{
			Key:         "port",
			Label:       "Port",
			Type:        plugin.FieldSelect,
			Placeholder: "Select a port",
			OptionsSource: &plugin.DataSource{
				RouteID: optionsRouteID,
				Params: map[string]string{
					"namespace": "${resource.namespace}",
					"name":      "${resource.name}",
				},
			},
		}},
	}}}
}

func ServiceOpenPorts(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	svc, err := s.clientset.CoreV1().Services(rc.Param("namespace")).Get(rc.Ctx, rc.Param("name"), metav1.GetOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	items := servicePortOptions(svc.Spec.Ports)
	return plugin.Page[plugin.Option]{Items: items, Total: ptr(len(items))}, nil
}

func PodOpenPorts(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	pod, err := s.clientset.CoreV1().Pods(rc.Param("namespace")).Get(rc.Ctx, rc.Param("name"), metav1.GetOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	items := podPortOptions(pod.Spec.Containers)
	return plugin.Page[plugin.Option]{Items: items, Total: ptr(len(items))}, nil
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

func servicePortOptions(ports []corev1.ServicePort) []plugin.Option {
	items := make([]plugin.Option, 0, len(ports))
	seen := map[string]bool{}
	for _, p := range ports {
		if p.Protocol != "" && p.Protocol != corev1.ProtocolTCP {
			continue
		}
		scheme, ok := webScheme(p)
		if !ok {
			scheme = defaultScheme(int(p.Port))
		}
		value := portSegment(p.Port, scheme)
		if seen[value] {
			continue
		}
		seen[value] = true
		label := fmt.Sprintf("%s %d/%s", strings.ToUpper(scheme), p.Port, protocolLabel(p.Protocol))
		if p.Name != "" {
			label = p.Name + " - " + label
		}
		target := p.TargetPort.String()
		if target != "" && target != "0" && target != strconv.Itoa(int(p.Port)) {
			label += " -> " + target
		}
		items = append(items, plugin.Option{Label: label, Value: value})
	}
	return items
}

func podPortOptions(containers []corev1.Container) []plugin.Option {
	var items []plugin.Option
	seen := map[string]bool{}
	for _, c := range containers {
		for _, p := range c.Ports {
			if p.Protocol != "" && p.Protocol != corev1.ProtocolTCP {
				continue
			}
			scheme, ok := webproxy.WebSchemeFromName(p.Name)
			if !ok {
				scheme = defaultScheme(int(p.ContainerPort))
			}
			value := portSegment(p.ContainerPort, scheme)
			if seen[value] {
				continue
			}
			seen[value] = true
			label := fmt.Sprintf("%s %d/%s", strings.ToUpper(scheme), p.ContainerPort, protocolLabel(p.Protocol))
			if p.Name != "" {
				label = p.Name + " - " + label
			}
			if c.Name != "" {
				label += " - " + c.Name
			}
			items = append(items, plugin.Option{Label: label, Value: value})
		}
	}
	return items
}

func protocolLabel(protocol corev1.Protocol) string {
	if protocol == "" {
		return string(corev1.ProtocolTCP)
	}
	return string(protocol)
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

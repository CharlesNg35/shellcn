package dockerengine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/plugins/shared/webproxy"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// ServeHTTPProxy reverse-proxies a browser to a container's or swarm service's
// web port, via `/{container|service}/{id}/{port}/{rest...}`.
func (s *Session) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	kind, id, portSeg, rest, ok := splitProxyPath(r.URL.Path)
	if !ok {
		http.Error(w, "unsupported proxy target", http.StatusBadRequest)
		return
	}
	prefix := fmt.Sprintf("%s/%s/%s/%s", plugin.RequestProxyPrefix(r), kind, id, portSeg)
	if strings.HasSuffix(r.URL.Path, "/"+webproxy.SWFile) {
		webproxy.ServeWorker(w, prefix)
		return
	}
	scheme, internalPort := "http", portSeg
	if p, https := strings.CutPrefix(portSeg, "https:"); https {
		scheme, internalPort = "https", p
	}
	host, dialPort, ok := s.proxyTarget(r.Context(), kind, id, internalPort)
	if !ok {
		http.Error(w, "target is not reachable from the gateway", http.StatusBadGateway)
		return
	}
	base := &url.URL{Scheme: scheme, Host: net.JoinHostPort(host, dialPort)}
	webproxy.Serve(w, r, webproxy.Options{
		Base:            base,
		Transport:       s.proxyTransport(),
		UpstreamPath:    "/" + rest,
		UpstreamRawPath: "/" + proxyRest(r.URL.EscapedPath()),
		PublicPrefix:    prefix,
		// No upstream-injected prefix: the app emits its own root-relative paths.
	})
}

// proxyTarget resolves where to dial: a container via its network, a service via
// the daemon host's routing-mesh port.
func (s *Session) proxyTarget(ctx context.Context, kind, id, internalPort string) (host, port string, ok bool) {
	if kind == "service" {
		return s.daemonProxyHost(), internalPort, true
	}
	res, err := s.cli.ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return "", "", false
	}
	return s.proxyDialTarget(res.Container.NetworkSettings, internalPort)
}

// daemonProxyHost is where the daemon's routing mesh is reachable: the remote tcp
// host, else loopback (local or agent).
func (s *Session) daemonProxyHost() string {
	if s.endpoint.network != "unix" {
		if h, _, err := net.SplitHostPort(s.endpoint.address); err == nil {
			return h
		}
	}
	return "127.0.0.1"
}

// splitProxyPath parses `/{container|service}/{id}/{port}/{rest}`.
func splitProxyPath(sub string) (kind, id, port, rest string, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(sub, "/"), "/", 4)
	if len(parts) < 3 || (parts[0] != "container" && parts[0] != "service") {
		return "", "", "", "", false
	}
	if len(parts) == 4 {
		rest = parts[3]
	}
	return parts[0], parts[1], parts[2], rest, true
}

// proxyRest returns the escaped `{rest}` segment so chunk filenames keep their
// original percent-encoding.
func proxyRest(escaped string) string {
	parts := strings.SplitN(strings.TrimPrefix(escaped, "/"), "/", 4)
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// proxyTransport dials the upstream container address over the session transport.
func (s *Session) proxyTransport() http.RoundTripper {
	return &http.Transport{
		DialContext: func(ctx context.Context, netw, addr string) (net.Conn, error) {
			return s.net.DialContext(ctx, netw, addr)
		},
		ForceAttemptHTTP2: false,
		//nolint:gosec // proxying to an internal container's (often self-signed) TLS.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

// proxyDialTarget resolves a container's dial address: its own IP for a local
// (unix) daemon, else the published host port on a remote (tcp) daemon.
func (s *Session) proxyDialTarget(ns *container.NetworkSettings, internalPort string) (host, port string, ok bool) {
	if s.endpoint.network == "unix" {
		ip, ok := containerIP(ns)
		return ip, internalPort, ok
	}
	hostPort, ok := publishedHostPort(ns, internalPort)
	if !ok {
		return "", "", false
	}
	daemonHost, _, err := net.SplitHostPort(s.endpoint.address)
	if err != nil {
		daemonHost = s.endpoint.address
	}
	return daemonHost, hostPort, true
}

// proxyPortCandidates lists a container's reachable TCP ports: exposed ports for a
// local (unix) daemon, only published ports for a remote (tcp) one.
func (s *Session) proxyPortCandidates(cfg *container.Config, ns *container.NetworkSettings) []int {
	if s.endpoint.network == "unix" {
		if cfg == nil {
			return nil
		}
		return tcpPortNums(cfg.ExposedPorts)
	}
	return publishedTCPPorts(ns)
}

// ContainerOpenSchema asks the renderer to select a reachable port before
// opening a browser proxy URL.
func ContainerOpenSchema(optionsRouteID string) *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{
		Name: "Open",
		Fields: []plugin.Field{{
			Key:         "port",
			Label:       "Port",
			Type:        plugin.FieldSelect,
			Placeholder: "Select a port",
			Help:        "Only TCP ports reachable from the gateway are shown.",
			OptionsSource: &plugin.DataSource{
				RouteID: optionsRouteID,
				Params:  map[string]string{"id": "${resource.uid}"},
			},
		}},
	}}}
}

// ContainerOpenPorts returns the port choices used by ContainerOpenSchema.
func ContainerOpenPorts(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerInspect(rc.Ctx, rc.Param("id"), dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	items := containerPortOptions(s.proxyPortCandidates(res.Container.Config, res.Container.NetworkSettings))
	return plugin.Page[plugin.Option]{Items: items, Total: ptr(len(items))}, nil
}

// containerIP returns a valid network IP of an inspected container (deterministic
// across its networks).
func containerIP(ns *container.NetworkSettings) (string, bool) {
	if ns == nil {
		return "", false
	}
	keys := make([]string, 0, len(ns.Networks))
	for k := range ns.Networks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if ep := ns.Networks[k]; ep != nil && ep.IPAddress.IsValid() {
			return ep.IPAddress.String(), true
		}
	}
	return "", false
}

// publishedHostPort returns the host-side port a container's internal TCP port is
// published on, if any.
func publishedHostPort(ns *container.NetworkSettings, internalPort string) (string, bool) {
	if ns == nil {
		return "", false
	}
	want, err := network.ParsePort(internalPort + "/tcp")
	if err != nil {
		return "", false
	}
	for _, b := range ns.Ports[want] {
		if b.HostPort != "" {
			return b.HostPort, true
		}
	}
	return "", false
}

// tcpPortNums returns the TCP port numbers from an exposed-port set.
func tcpPortNums(set network.PortSet) []int {
	out := make([]int, 0, len(set))
	for p := range set {
		if p.Proto() == network.TCP {
			out = append(out, int(p.Num()))
		}
	}
	return out
}

// publishedTCPPorts returns the container's internal TCP ports that have a host
// binding (the ones reachable through a remote daemon).
func publishedTCPPorts(ns *container.NetworkSettings) []int {
	if ns == nil {
		return nil
	}
	out := make([]int, 0, len(ns.Ports))
	for p, bindings := range ns.Ports {
		if p.Proto() != network.TCP {
			continue
		}
		for _, b := range bindings {
			if b.HostPort != "" {
				out = append(out, int(p.Num()))
				break
			}
		}
	}
	return out
}

func portSegmentForNumber(port int) string {
	seg := strconv.Itoa(port)
	if webproxy.IsTLSPort(port) {
		return "https:" + seg
	}
	return seg
}

func containerPortOptions(ports []int) []plugin.Option {
	if len(ports) == 0 {
		return nil
	}
	sort.Ints(ports)
	items := make([]plugin.Option, 0, len(ports))
	seen := map[int]bool{}
	for _, port := range ports {
		if seen[port] {
			continue
		}
		seen[port] = true
		scheme := "HTTP"
		if webproxy.IsTLSPort(port) {
			scheme = "HTTPS"
		}
		items = append(items, plugin.Option{
			Label: fmt.Sprintf("%s %d/tcp", scheme, port),
			Value: portSegmentForNumber(port),
		})
	}
	return items
}

// ContainerProxyURL returns the gateway URL that proxies to a container's web
// port for an "Open in browser" link, picking a likely web port when none given.
func ContainerProxyURL(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	id := rc.Param("id")
	portSeg := rc.Param("port")
	if portSeg == "" {
		res, err := s.cli.ContainerInspect(rc.Ctx, id, dockerclient.ContainerInspectOptions{})
		if err != nil {
			return nil, DockerErr(err)
		}
		portSeg, err = pickWebPort(s.proxyPortCandidates(res.Container.Config, res.Container.NetworkSettings))
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"url": rc.ProxyURL("container", shortID(id), portSeg)}, nil
}

// pickWebPort picks the lowest reachable TCP port (Docker exposes only the
// number, so this is best-effort). 443/8443 proxy over TLS.
func pickWebPort(ports []int) (string, error) {
	if len(ports) == 0 {
		return "", fmt.Errorf("%w: no reachable TCP ports", plugin.ErrInvalidInput)
	}
	sort.Ints(ports)
	return portSegmentForNumber(ports[0]), nil
}

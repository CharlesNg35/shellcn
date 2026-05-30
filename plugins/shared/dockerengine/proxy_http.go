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

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/webproxy"
)

// ServeHTTPProxy reverse-proxies a browser request straight to a container's web
// port — the Docker equivalent of opening a Kubernetes Service in the browser.
// The incoming path is `/container/{id}/{port}/{rest...}`; we resolve the
// container's network IP, dial it over the session transport, and hand the
// response to the shared rewriter. The gateway must be able to route to the
// container's network (a local daemon); remote/agent reach is a separate matter.
func (s *Session) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	id, portSeg, rest, ok := splitProxyPath(r.URL.Path)
	if !ok {
		http.Error(w, "unsupported proxy target", http.StatusBadRequest)
		return
	}
	prefix := fmt.Sprintf("/api/connections/%s/proxy/container/%s/%s", s.connID, id, portSeg)
	if strings.HasSuffix(r.URL.Path, "/"+webproxy.SWFile) {
		webproxy.ServeWorker(w, prefix)
		return
	}
	res, err := s.cli.ContainerInspect(r.Context(), id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		http.Error(w, "inspect: "+err.Error(), http.StatusBadGateway)
		return
	}
	scheme, internalPort := "http", portSeg
	if p, https := strings.CutPrefix(portSeg, "https:"); https {
		scheme, internalPort = "https", p
	}
	host, dialPort, ok := s.proxyDialTarget(res.Container.NetworkSettings, internalPort)
	if !ok {
		http.Error(w, "container port is not reachable from the gateway", http.StatusBadGateway)
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

// splitProxyPath parses `/container/{id}/{port}/{rest}`.
func splitProxyPath(sub string) (id, port, rest string, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(sub, "/"), "/", 4)
	if len(parts) < 3 || parts[0] != "container" {
		return "", "", "", false
	}
	if len(parts) == 4 {
		rest = parts[3]
	}
	return parts[1], parts[2], rest, true
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

// proxyDialTarget resolves where to dial for a container's internal port, honoring
// the endpoint. A local (unix) daemon shares the host's network, so the container's
// own IP is reachable; a remote (tcp) daemon is reached through the published host
// port on the daemon host (the only container port routable from the gateway).
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

// proxyPortCandidates lists a container's reachable internal TCP ports: any
// exposed port for a local (unix) daemon, but only published ports for a remote
// (tcp) daemon, since unpublished bridge ports aren't routable from the gateway.
func (s *Session) proxyPortCandidates(cfg *container.Config, ns *container.NetworkSettings) []int {
	if s.endpoint.network == "unix" {
		if cfg == nil {
			return nil
		}
		return tcpPortNums(cfg.ExposedPorts)
	}
	return publishedTCPPorts(ns)
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
	u := fmt.Sprintf("/api/connections/%s/proxy/container/%s/%s/", url.PathEscape(s.connID), url.PathEscape(id), portSeg)
	return map[string]any{"url": u}, nil
}

// pickWebPort chooses a container's web port from its reachable TCP ports: a
// conventional web port if present, else the lowest. 443/8443 proxy over TLS.
func pickWebPort(ports []int) (string, error) {
	if len(ports) == 0 {
		return "", fmt.Errorf("%w: container exposes no reachable TCP ports", plugin.ErrInvalidInput)
	}
	sort.Ints(ports)
	pick := ports[0]
	for _, want := range []int{80, 8080, 3000, 8000, 5000} {
		if contains(ports, want) {
			pick = want
			break
		}
	}
	seg := strconv.Itoa(pick)
	if pick == 443 || pick == 8443 {
		return "https:" + seg, nil
	}
	return seg, nil
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

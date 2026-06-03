package kubernetes

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/streaming/pkg/httpstream"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// proxyTransport returns an http.Transport whose connections are port-forward
// tunnels to a pod port. Reaching the workload through the port-forward
// subresource keeps the hop at L4, so the app's own Authorization/cookies survive
// to the backend — unlike the API server's HTTP proxy, which strips them. It is
// built once per target and reused so the transport can pool tunnels.
func (s *Session) proxyTransport(ns, pod string, port int, https bool) (*http.Transport, error) {
	key := fmt.Sprintf("%s/%s/%d/%t", ns, pod, port, https)
	s.mu.Lock()
	if s.pfTransports == nil {
		s.pfTransports = map[string]*http.Transport{}
	} else if tr := s.pfTransports[key]; tr != nil {
		s.mu.Unlock()
		return tr, nil
	}
	s.mu.Unlock()

	dialer, err := s.portForwardDialer(ns, pod)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		DialContext:         func(context.Context, string, string) (net.Conn, error) { return dialPodPort(dialer, port) },
		MaxIdleConns:        8,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
	}
	if https {
		// The tunnel terminates inside the pod, where the served certificate never
		// matches the dialed address; the API server already authenticated the hop.
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	s.mu.Lock()
	s.pfTransports[key] = tr
	s.mu.Unlock()
	return tr, nil
}

// portForwardDialer builds a fallback (WebSocket → SPDY) port-forward dialer
// against the upgrade config (the loopback bridge for agent transport, the
// kubeconfig for direct), mirroring how exec resolves its executor.
func (s *Session) portForwardDialer(ns, pod string) (httpstream.Dialer, error) {
	cfg, err := s.upgradeConfig()
	if err != nil {
		return nil, err
	}
	cfg.GroupVersion = &corev1.SchemeGroupVersion
	cfg.APIPath = "/api"
	cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	client, err := rest.RESTClientFor(cfg)
	if err != nil {
		return nil, err
	}
	u := client.Post().Resource("pods").Namespace(ns).Name(pod).SubResource("portforward").URL()

	roundTripper, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return nil, err
	}
	spdyDialer := spdy.NewDialerForStreaming(upgrader, &http.Client{Transport: roundTripper}, "POST", u)
	wsDialer, err := portforward.NewSPDYOverWebsocketDialerForStreaming(u, cfg)
	if err != nil {
		return spdyDialer, nil
	}
	return portforward.NewFallbackDialerForStreaming(wsDialer, spdyDialer, func(err error) bool {
		return httpstream.IsUpgradeFailure(err)
	}), nil
}

// dialPodPort opens one port-forward data stream to the pod port and adapts it to
// a net.Conn. The protocol requires an error stream (created first, never written)
// alongside the data stream; the server reports failures on it.
func dialPodPort(dialer httpstream.Dialer, port int) (net.Conn, error) {
	conn, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set(corev1.StreamType, corev1.StreamTypeError)
	h.Set(corev1.PortHeader, strconv.Itoa(port))
	h.Set(corev1.PortForwardRequestIDHeader, "0")
	errStream, err := conn.CreateStream(h)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = errStream.Close()
	h.Set(corev1.StreamType, corev1.StreamTypeData)
	dataStream, err := conn.CreateStream(h)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	c := &tunnelConn{conn: conn, data: dataStream, errStream: errStream}
	go c.watch()
	return c, nil
}

// tunnelConn adapts a port-forward data stream to net.Conn for an http.Transport.
type tunnelConn struct {
	conn      httpstream.Connection
	data      httpstream.Stream
	errStream httpstream.Stream
	closeOnce sync.Once
}

func (c *tunnelConn) Read(b []byte) (int, error)  { return c.data.Read(b) }
func (c *tunnelConn) Write(b []byte) (int, error) { return c.data.Write(b) }

func (c *tunnelConn) Close() error {
	c.closeOnce.Do(func() {
		_ = c.data.Close()
		c.conn.RemoveStreams(c.data, c.errStream)
	})
	return c.conn.Close()
}

// watch tears down the data stream when the server reports a forwarding failure on
// the error stream, so the reverse proxy sees the connection drop.
func (c *tunnelConn) watch() {
	if msg, _ := io.ReadAll(c.errStream); len(msg) > 0 {
		_ = c.data.Reset()
	}
}

func (*tunnelConn) LocalAddr() net.Addr              { return tunnelAddr{} }
func (*tunnelConn) RemoteAddr() net.Addr             { return tunnelAddr{} }
func (*tunnelConn) SetDeadline(time.Time) error      { return nil }
func (*tunnelConn) SetReadDeadline(time.Time) error  { return nil }
func (*tunnelConn) SetWriteDeadline(time.Time) error { return nil }

type tunnelAddr struct{}

func (tunnelAddr) Network() string { return "k8s-portforward" }
func (tunnelAddr) String() string  { return "k8s-portforward" }

// proxyPodTarget resolves a proxy target to a concrete pod and port. A pod is
// itself; a service is resolved to a ready backing pod and its target port via the
// endpoint slices, since port-forward attaches only to pods.
func (s *Session) proxyPodTarget(ctx context.Context, kind, ns, name string, port int) (podNS, podName string, podPort int, err error) {
	if kind == "pods" {
		return ns, name, port, nil
	}
	svc, err := s.clientset.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", "", 0, apiErr(err)
	}
	portName, matched := "", false
	for _, sp := range svc.Spec.Ports {
		if int(sp.Port) == port {
			portName, matched = sp.Name, true
			break
		}
	}
	if !matched {
		return "", "", 0, fmt.Errorf("%w: service %q exposes no port %d", plugin.ErrInvalidInput, name, port)
	}
	slices, err := s.clientset.DiscoveryV1().EndpointSlices(ns).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + name,
	})
	if err != nil {
		return "", "", 0, apiErr(err)
	}
	for i := range slices.Items {
		sl := &slices.Items[i]
		target := endpointPort(sl.Ports, portName)
		if target < 0 {
			continue
		}
		for j := range sl.Endpoints {
			ep := &sl.Endpoints[j]
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" && ep.TargetRef.Name != "" {
				podNS = ep.TargetRef.Namespace
				if podNS == "" {
					podNS = ns
				}
				return podNS, ep.TargetRef.Name, target, nil
			}
		}
	}
	return "", "", 0, fmt.Errorf("%w: service %q has no ready backing pod for port %d", plugin.ErrUnavailable, name, port)
}

// endpointPort returns the numeric port for the named service port within an
// endpoint slice (-1 when absent).
func endpointPort(ports []discoveryv1.EndpointPort, name string) int {
	for i := range ports {
		p := &ports[i]
		named := p.Name != nil && *p.Name == name
		unnamed := p.Name == nil && name == ""
		if (named || unnamed) && p.Port != nil {
			return int(*p.Port)
		}
	}
	return -1
}

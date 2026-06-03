package kubernetes

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/charlesng35/shellcn/plugins/shared/loopback"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Session is a live connection to one Kubernetes cluster. It holds every
// per-connection client; the plugin struct stays stateless. Heavy machinery
// (informers) is opened lazily and guarded by mu.
type Session struct {
	rest      *rest.Config
	clientset *kubeclient.Clientset
	dyn       *dynamic.DynamicClient
	disco     discovery.CachedDiscoveryInterface
	mapper    meta.RESTMapper
	metrics   *metricsclient.Clientset

	connID     string
	defaultNS  string
	metricsSrc string
	promURL    string

	transport plugin.Transport
	net       plugin.NetTransport

	mu           sync.Mutex
	stopCh       chan struct{}
	stopped      bool
	bridge       *loopback.Bridge           // lazy, agent transport only
	pfTransports map[string]*http.Transport // lazy; pools port-forward tunnels per pod port
}

// Connect builds the REST config for the connection's transport and wires the
// typed, dynamic, discovery, RESTMapper, and metrics clients over it.
func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	restCfg, err := buildRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	restCfg.QPS = 50
	restCfg.Burst = 100
	restCfg.UserAgent = plugin.DefaultClientName

	clientset, err := kubeclient.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: kubernetes client: %v", plugin.ErrUnavailable, err)
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: dynamic client: %v", plugin.ErrUnavailable, err)
	}
	discoClient, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: discovery client: %v", plugin.ErrUnavailable, err)
	}
	cached := memory.NewMemCacheClient(discoClient)
	metricsClient, err := metricsclient.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: metrics client: %v", plugin.ErrUnavailable, err)
	}

	s := &Session{
		rest:       restCfg,
		clientset:  clientset,
		dyn:        dyn,
		disco:      cached,
		mapper:     restmapper.NewDeferredDiscoveryRESTMapper(cached),
		metrics:    metricsClient,
		connID:     cfg.ConnectionID,
		defaultNS:  strings.TrimSpace(cfg.String("namespace")),
		metricsSrc: metricsSourceOrDefault(cfg.String("metrics_source")),
		promURL:    strings.TrimSpace(cfg.String("prometheus_url")),
		transport:  cfg.Transport,
		net:        cfg.Net,
		stopCh:     make(chan struct{}),
	}
	if err := s.HealthCheck(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func metricsSourceOrDefault(v string) string {
	switch v {
	case metricsServer, metricsProm, metricsNone:
		return v
	default:
		return metricsServer
	}
}

// buildRESTConfig produces a rest.Config for the connection's transport. Agent
// transport requires the L7 (http_proxy) endpoint: the agent injects the
// target's credentials, so the gateway speaks plain HTTP over the tunnel. Direct
// transport builds the config from the user's kubeconfig.
func buildRESTConfig(cfg plugin.ConnectConfig) (*rest.Config, error) {
	if cfg.Transport == plugin.TransportAgent {
		baseURL, rt, ok := cfg.Net.HTTP()
		if !ok {
			return nil, fmt.Errorf("%w: agent transport must run in http_proxy (L7) mode", plugin.ErrUnavailable)
		}
		return &rest.Config{Host: baseURL, Transport: rt}, nil
	}

	raw := strings.TrimSpace(cfg.String("kubeconfig"))
	if raw == "" {
		return nil, fmt.Errorf("%w: kubeconfig is required for direct transport", plugin.ErrInvalidInput)
	}
	apiCfg, err := clientcmd.Load([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: parse kubeconfig: %v", plugin.ErrInvalidInput, err)
	}
	overrides := &clientcmd.ConfigOverrides{}
	if ctxName := strings.TrimSpace(cfg.String("context")); ctxName != "" {
		overrides.CurrentContext = ctxName
	}
	restCfg, err := clientcmd.NewDefaultClientConfig(*apiCfg, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: build kubeconfig: %v", plugin.ErrInvalidInput, err)
	}
	// An exec credential plugin would run an arbitrary binary on the gateway when
	// the client authenticates — reject it so an uploaded kubeconfig can never
	// execute code here.
	if restCfg.ExecProvider != nil {
		return nil, fmt.Errorf("%w: kubeconfig exec credential plugins are not allowed; use a token or client certificate", plugin.ErrInvalidInput)
	}
	return restCfg, nil
}

// HealthCheck probes connectivity and auth with a context-bound /version call
// (allowed for every principal, so it never trips on a scoped credential).
func (s *Session) HealthCheck(ctx context.Context) error {
	if err := s.clientset.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Error(); err != nil {
		return fmt.Errorf("%w: kubernetes: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

// OpenChannel is implemented in step 3 (pod logs/exec/port-forward); core
// resources stream via informer-backed watch routes, not channels.
func (s *Session) OpenChannel(_ context.Context, _ plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

// Close stops lazily-started machinery (watches, the upgrade bridge).
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.stopped = true
		close(s.stopCh)
	}
	if s.bridge != nil {
		_ = s.bridge.Close()
		s.bridge = nil
	}
	for _, tr := range s.pfTransports {
		tr.CloseIdleConnections()
	}
	s.pfTransports = nil
	return nil
}

// upgradeConfig returns a rest.Config usable for SPDY/WebSocket upgrades
// (exec, attach, port-forward). For direct transport that is the kubeconfig
// config itself. For agent transport, client-go upgraders ignore a custom
// dialer, so we front the tunnel with a lazily-started loopback bridge and point
// the config at it.
func (s *Session) upgradeConfig() (*rest.Config, error) {
	if s.transport != plugin.TransportAgent {
		return rest.CopyConfig(s.rest), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil, fmt.Errorf("%w: session closed", plugin.ErrUnavailable)
	}
	if s.bridge == nil {
		b, err := loopback.New(func(ctx context.Context) (net.Conn, error) {
			return s.net.DialContext(ctx, "tcp", agentUpgradeAddr)
		})
		if err != nil {
			return nil, fmt.Errorf("%w: upgrade bridge: %v", plugin.ErrUnavailable, err)
		}
		s.bridge = b
	}
	return &rest.Config{Host: s.bridge.Host()}, nil
}

// agentUpgradeAddr is a placeholder address the agent tunnel dialer ignores.
const agentUpgradeAddr = plugin.AgentInternalAddress

// Clientset exposes the typed client for built-in API groups.
func (s *Session) Clientset() *kubeclient.Clientset { return s.clientset }

// Dynamic exposes the unstructured client used for every kind, including CRDs.
func (s *Session) Dynamic() dynamic.Interface { return s.dyn }

// Mapper resolves a GroupVersionKind to its REST resource (and scope).
func (s *Session) Mapper() meta.RESTMapper { return s.mapper }

// Discovery exposes cached API discovery (kinds, CRDs, server version).
func (s *Session) Discovery() discovery.CachedDiscoveryInterface { return s.disco }

// Metrics exposes the metrics.k8s.io client (may be unavailable on a cluster
// without metrics-server; callers degrade gracefully).
func (s *Session) Metrics() *metricsclient.Clientset { return s.metrics }

// DefaultNamespace is the connection's configured default ("" = all namespaces).
func (s *Session) DefaultNamespace() string { return s.defaultNS }

// Unwrap recovers the concrete Session from a plugin.Session, looking through a
// wrapper (e.g. the recording layer) that exposes Session().
func Unwrap(sess plugin.Session) (*Session, error) {
	if s, ok := sess.(*Session); ok {
		return s, nil
	}
	type sessionGetter interface{ Session() plugin.Session }
	if h, ok := sess.(sessionGetter); ok {
		if s, ok := h.Session().(*Session); ok {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: Kubernetes session unavailable", plugin.ErrUnavailable)
}

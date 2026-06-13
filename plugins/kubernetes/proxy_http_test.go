package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestSplitProxyTarget(t *testing.T) {
	cases := []struct {
		in                            string
		kind, ns, name, portSeg, rest string
		ok                            bool
	}{
		{"/services/default/web/80/", "services", "default", "web", "80", "", true},
		{"/pods/default/api/8080/healthz", "pods", "default", "api", "8080", "healthz", true},
		{"/services/mon/graf/https:8443/x/y", "services", "mon", "graf", "https:8443", "x/y", true},
		{"/configmaps/default/cm", "", "", "", "", "", false},
		{"/services/default/web", "", "", "", "", "", false},
	}
	for _, c := range cases {
		kind, ns, name, portSeg, rest, ok := splitProxyTarget(c.in)
		if kind != c.kind || ns != c.ns || name != c.name || portSeg != c.portSeg || rest != c.rest || ok != c.ok {
			t.Errorf("splitProxyTarget(%q) = %q,%q,%q,%q,%q,%v", c.in, kind, ns, name, portSeg, rest, ok)
		}
	}
}

func TestSchemePort(t *testing.T) {
	cases := []struct {
		in     string
		scheme string
		port   int
		ok     bool
	}{
		{"80", "http", 80, true},
		{"https:8443", "https", 8443, true},
		{"0", "", 0, false},
		{"abc", "", 0, false},
		{"99999", "", 0, false},
	}
	for _, c := range cases {
		scheme, port, ok := schemePort(c.in)
		if scheme != c.scheme || port != c.port || ok != c.ok {
			t.Errorf("schemePort(%q) = %q,%d,%v", c.in, scheme, port, ok)
		}
	}
}

// A service resolves to a ready backing pod and the pod-side target port (so the
// port-forward attaches to a pod); a pod resolves to itself.
func TestProxyPodTarget(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/services/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Service", "metadata": obj{"name": "web", "namespace": "default"},
			"spec": obj{"ports": []any{obj{"name": "http", "port": int64(80), "targetPort": int64(8080)}}},
		})
	})
	mux.HandleFunc("/apis/discovery.k8s.io/v1/namespaces/default/endpointslices", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "discovery.k8s.io/v1", "kind": "EndpointSliceList",
			"items": []any{obj{
				"metadata": obj{"name": "web-abc", "namespace": "default", "labels": obj{"kubernetes.io/service-name": "web"}},
				"ports":    []any{obj{"name": "http", "port": int64(8080)}},
				"endpoints": []any{obj{
					"addresses":  []any{"10.1.2.3"},
					"conditions": obj{"ready": true},
					"targetRef":  obj{"kind": "Pod", "name": "web-xyz", "namespace": "default"},
				}},
			}},
		})
	})
	s := connectTo(t, mux).(*Session)

	podNS, podName, podPort, err := s.proxyPodTarget(context.Background(), "services", "default", "web", 80)
	if err != nil {
		t.Fatalf("resolve service: %v", err)
	}
	if podNS != "default" || podName != "web-xyz" || podPort != 8080 {
		t.Fatalf("service resolve = %s/%s:%d; want default/web-xyz:8080", podNS, podName, podPort)
	}

	pn, pp, port, err := s.proxyPodTarget(context.Background(), "pods", "default", "api", 9090)
	if err != nil || pn != "default" || pp != "api" || port != 9090 {
		t.Fatalf("pod self-resolve = %s/%s:%d,%v", pn, pp, port, err)
	}

	if _, _, _, err := s.proxyPodTarget(context.Background(), "services", "default", "web", 12345); err == nil {
		t.Error("an unexposed service port should error")
	}
}

func TestServeHTTPProxyServesServiceWorker(t *testing.T) {
	sess := connectTo(t, http.NewServeMux()).(*Session)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/services/default/web/80/__shellcn_sw.js", nil)
	req.Header.Set(plugin.ProxyPrefixHeader, "/api/connections/c1/proxy")
	sess.ServeHTTPProxy(rec, req)

	prefix := "/api/connections/c1/proxy/services/default/web/80"
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("worker content-type = %q", ct)
	}
	if got := rec.Header().Get("Service-Worker-Allowed"); got != prefix+"/" {
		t.Fatalf("worker scope header = %q", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, prefix) || !strings.Contains(body, `addEventListener("fetch"`) {
		t.Fatalf("worker body unexpected: %s", body)
	}
}

func TestServiceProxyURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/services/web", func(w http.ResponseWriter, _ *http.Request) {
		// A non-web port first, then http — selection must prefer the http port.
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Service", "metadata": obj{"name": "web", "namespace": "default"},
			"spec": obj{"ports": []any{obj{"name": "grpc", "port": int64(9000)}, obj{"name": "http", "port": int64(8080)}}},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/services/secure", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Service", "metadata": obj{"name": "secure", "namespace": "default"},
			"spec": obj{"ports": []any{obj{"name": "https", "port": int64(8443)}}},
		})
	})
	sess := connectTo(t, mux)

	out, err := ServiceProxyURL(rc(sess, map[string]string{"namespace": "default", "name": "web"}))
	if err != nil {
		t.Fatalf("proxy url: %v", err)
	}
	if url, _ := out.(map[string]any)["url"].(string); url != "/api/connections/c1/proxy/services/default/web/8080/" {
		t.Fatalf("http port should win: %q", url)
	}

	out, err = ServiceProxyURL(rc(sess, map[string]string{"namespace": "default", "name": "secure"}))
	if err != nil {
		t.Fatalf("proxy url (https): %v", err)
	}
	if url, _ := out.(map[string]any)["url"].(string); url != "/api/connections/c1/proxy/services/default/secure/https:8443/" {
		t.Fatalf("https port segment wrong: %q", url)
	}
}

func TestPodProxyURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod", "metadata": obj{"name": "web", "namespace": "default"},
			"spec": obj{"containers": []any{obj{"name": "app", "ports": []any{
				obj{"name": "metrics", "containerPort": int64(9090)},
				obj{"name": "http", "containerPort": int64(8080)},
			}}}},
		})
	})
	sess := connectTo(t, mux)

	out, err := PodProxyURL(rc(sess, map[string]string{"namespace": "default", "name": "web"}))
	if err != nil {
		t.Fatalf("pod proxy url: %v", err)
	}
	if url, _ := out.(map[string]any)["url"].(string); url != "/api/connections/c1/proxy/pods/default/web/8080/" {
		t.Fatalf("http container port should win: %q", url)
	}
}

func TestOpenURLRoutesValidateIdentityBeforeLookup(t *testing.T) {
	sess := connectTo(t, http.NewServeMux())
	cases := []struct {
		name   string
		handle func(*plugin.RequestContext) (any, error)
		params map[string]string
	}{
		{"service namespace", ServiceProxyURL, map[string]string{"name": "web"}},
		{"service name", ServiceProxyURL, map[string]string{"namespace": "default"}},
		{"pod namespace", PodProxyURL, map[string]string{"name": "web"}},
		{"pod name", PodProxyURL, map[string]string{"namespace": "default"}},
		{"service ports namespace", ServiceOpenPorts, map[string]string{"name": "web"}},
		{"pod ports name", PodOpenPorts, map[string]string{"namespace": "default"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.handle(rc(sess, c.params))
			if !errors.Is(err, plugin.ErrInvalidInput) {
				t.Fatalf("error = %v, want invalid input", err)
			}
		})
	}
}

func TestOpenPortOptions(t *testing.T) {
	appProtocol := "https"
	serviceOptions := servicePortOptions([]corev1.ServicePort{
		{Name: "grpc", Port: 9000, TargetPort: intstr.FromInt32(9001)},
		{Name: "web", Port: 8443, AppProtocol: &appProtocol},
	})
	if len(serviceOptions) != 2 {
		t.Fatalf("service options = %#v", serviceOptions)
	}
	if serviceOptions[0].Value != "9000" || !strings.Contains(serviceOptions[0].Label, "9000/TCP -> 9001") {
		t.Fatalf("service option 0 unexpected: %+v", serviceOptions[0])
	}
	if serviceOptions[1].Value != "https:8443" || !strings.Contains(serviceOptions[1].Label, "web") {
		t.Fatalf("service option 1 unexpected: %+v", serviceOptions[1])
	}

	podOptions := podPortOptions([]corev1.Container{{
		Name: "app",
		Ports: []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 9090},
			{Name: "https", ContainerPort: 8443},
		},
	}})
	if len(podOptions) != 2 {
		t.Fatalf("pod options = %#v", podOptions)
	}
	if podOptions[0].Value != "9090" || !strings.Contains(podOptions[0].Label, "app") {
		t.Fatalf("pod option 0 unexpected: %+v", podOptions[0])
	}
	if podOptions[1].Value != "https:8443" {
		t.Fatalf("pod option 1 unexpected: %+v", podOptions[1])
	}
}

func TestServiceOpenActionIsURLTarget(t *testing.T) {
	for _, a := range New().Manifest().Actions {
		if a.ID == "kubernetes.service.open" {
			if a.Open != plugin.OpenURL {
				t.Fatalf("service open action should use OpenURL, got %q", a.Open)
			}
			return
		}
	}
	t.Fatal("kubernetes.service.open action not declared")
}

func TestOpenRoutesDeclarePortSelectors(t *testing.T) {
	routes := Routes()
	for _, routeID := range []string{"kubernetes.service.open", "kubernetes.pod.open"} {
		var route *plugin.Route
		for i := range routes {
			if routes[i].ID == routeID {
				route = &routes[i]
				break
			}
		}
		if route == nil || route.Input == nil {
			t.Fatalf("%s route should declare port input: %+v", routeID, route)
		}
		field := route.Input.Groups[0].Fields[0]
		if field.Type != plugin.FieldSelect || field.OptionsSource == nil {
			t.Fatalf("%s port field should be sourced select: %+v", routeID, field)
		}
		if field.Required {
			t.Fatalf("%s port field is a URL route param and must not make the GET body schema required", routeID)
		}
		if err := route.Input.ValidateValues(map[string]any{}, nil); err != nil {
			t.Fatalf("%s input should allow fallback port selection: %v", routeID, err)
		}
	}
}

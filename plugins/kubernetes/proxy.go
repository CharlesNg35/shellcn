package kubernetes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charlesng/shellcn/internal/plugin"
)

const portForwardKind = "portforward"

// ProxyRequest is the http_client request body for the Port Forwarding console.
type ProxyRequest struct {
	Method  string `json:"method"`
	URL     string `json:"url"`
	Headers []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"headers"`
	Body string `json:"body"`
}

// ProxyExecute forwards an HTTP request to an in-cluster Service or Pod port via
// the API server's proxy subresource — the web-feasible equivalent of a
// port-forward. It works over both transports (it rides the API connection) and
// is governed by the same RBAC as every other call.
func ProxyExecute(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var in ProxyRequest
	if err := rc.Bind(&in); err != nil {
		return nil, err
	}
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
	default:
		return nil, fmt.Errorf("%w: unsupported method %q", plugin.ErrInvalidInput, in.Method)
	}
	path := strings.TrimSpace(in.URL)
	if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/apis/") {
		return nil, fmt.Errorf("%w: proxy path must be an API server path (…/services/<svc>:<port>/proxy/…)", plugin.ErrInvalidInput)
	}

	req := s.clientset.Discovery().RESTClient().Verb(method).AbsPath(path)
	if in.Body != "" {
		req = req.Body([]byte(in.Body))
	}
	for _, h := range in.Headers {
		if k := strings.TrimSpace(h.Key); k != "" && !strings.EqualFold(k, "Host") {
			req = req.SetHeader(k, h.Value)
		}
	}
	res := req.Do(rc.Ctx)
	var code int
	res.StatusCode(&code)
	body, _ := res.Raw()
	return map[string]any{
		"ok":     true,
		"status": code,
		"body":   decodeProxyBody(body),
	}, nil
}

func decodeProxyBody(body []byte) any {
	var v any
	if json.Unmarshal(body, &v) == nil {
		return v
	}
	return string(body)
}

// PortForwardList backs the Port Forwarding console (a single console row).
func PortForwardList(_ *plugin.RequestContext) (any, error) {
	return plugin.Page[Row]{Items: []Row{{
		"name": "HTTP proxy", "uid": portForwardKind,
		"ref": plugin.ResourceRef{Kind: portForwardKind, Name: "HTTP proxy", UID: portForwardKind},
	}}, Total: ptr(1)}, nil
}

// TreePortForward returns the single Port Forwarding console node.
func TreePortForward(_ *plugin.RequestContext) (any, error) {
	return plugin.Page[plugin.TreeNode]{Items: []plugin.TreeNode{{
		Key:   "portforward:console",
		Label: "HTTP proxy",
		Icon:  lucide("arrow-left-right"),
		Ref:   &plugin.ResourceRef{Kind: portForwardKind, Name: "HTTP proxy", UID: portForwardKind},
		Leaf:  true,
	}}, Total: ptr(1)}, nil
}

// portForwardResourceType is the Port Forwarding console: an HTTP client that
// proxies to any in-cluster Service/Pod port through the API server.
func portForwardResourceType() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:    portForwardKind,
		Title:   "Port Forwarding",
		List:    plugin.DataSource{RouteID: "kubernetes.portforward.list"},
		Columns: []plugin.Column{nameCol()},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "Port Forwarding"},
			Tabs: []plugin.Tab{
				{
					Key: "proxy", Label: "HTTP proxy", Icon: lucide("arrow-left-right"), Panel: plugin.PanelHTTPClient,
					Config: plugin.HTTPClientConfig{
						ExecuteRouteID: "kubernetes.proxy.execute",
						Methods:        []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
						DefaultMethod:  "GET",
						DefaultURL:     "/api/v1/namespaces/default/services/my-service:80/proxy/",
					}.Map(),
				},
			},
		},
	}
}

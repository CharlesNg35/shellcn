package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Cluster-wide PromQL (node-exporter metrics) for used CPU cores and memory bytes.
const (
	promCPUUsedExpr = `sum(rate(node_cpu_seconds_total{mode!="idle"}[2m]))`
	promMemUsedExpr = `sum(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes)`
)

// promClusterUsage queries the configured Prometheus for cluster CPU (cores) and
// memory (bytes) usage. It reaches Prometheus through the API server's service
// proxy, so it works identically over direct and agent transport.
func (s *Session) promClusterUsage(ctx context.Context) (cpuCores, memBytes float64, ok bool) {
	cpu, cok := s.promQuery(ctx, promCPUUsedExpr)
	mem, mok := s.promQuery(ctx, promMemUsedExpr)
	if !cok && !mok {
		return 0, 0, false
	}
	return cpu, mem, true
}

// promQuery runs an instant PromQL query and returns the summed scalar result.
func (s *Session) promQuery(ctx context.Context, expr string) (float64, bool) {
	ns, svc, port, ok := parsePromService(s.promURL)
	if !ok {
		return 0, false
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/%s:%s/proxy/api/v1/query", ns, svc, port)
	raw, err := s.clientset.Discovery().RESTClient().Get().AbsPath(path).Param("query", expr).DoRaw(ctx)
	if err != nil {
		return 0, false
	}
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || resp.Status != "success" {
		return 0, false
	}
	var sum float64
	var found bool
	for _, r := range resp.Data.Result {
		if len(r.Value) == 2 {
			if str, isStr := r.Value[1].(string); isStr {
				if f, err := strconv.ParseFloat(str, 64); err == nil {
					sum += f
					found = true
				}
			}
		}
	}
	return sum, found
}

// parsePromService extracts the service ref from an in-cluster Prometheus URL
// like "http://prometheus.monitoring.svc:9090" → (monitoring, prometheus, 9090).
func parsePromService(raw string) (namespace, service, port string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", false
	}
	host := raw
	port = "80"
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		host = u.Hostname()
		if u.Port() != "" {
			port = u.Port()
		}
	} else if h, p, err := splitHostPort(raw); err == nil {
		host, port = h, p
	}
	// host: <service>.<namespace>.svc[.cluster.local]
	labels := strings.Split(host, ".")
	if len(labels) < 3 || labels[2] != "svc" {
		return "", "", "", false
	}
	return labels[1], labels[0], port, true
}

func splitHostPort(s string) (host, port string, err error) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, "80", nil
	}
	return s[:i], s[i+1:], nil
}

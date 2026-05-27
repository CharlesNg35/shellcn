package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/transport"
	"github.com/charlesng/shellcn/plugins/shared/escompat"
)

func TestElasticsearchPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_ELASTICSEARCH_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_ELASTICSEARCH_INTEGRATION=1 to run against Elasticsearch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	cfg := elasticsearchIntegrationConfig(ctx, t)
	p := New()
	sess, err := p.Connect(ctx, plugin.ConnectConfig{Config: cfg, Net: transport.NewDirectForConnection(models.Connection{Config: cfg})})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	routes := routeMap(p.Routes())
	index := "shellcn-it-" + time.Now().UTC().Format("20060102150405")
	createBody, _ := json.Marshal(map[string]any{
		"name": index,
		"settings": map[string]any{
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{"properties": map[string]any{
			"name": map[string]any{"type": "keyword"},
			"age":  map[string]any{"type": "integer"},
		}},
	})
	call(t, ctx, routes["elasticsearch.index.create"], sess, nil, nil, createBody)
	defer callNoFail(context.Background(), routes["elasticsearch.index.delete"], sess, map[string]string{"index": index})

	docBody, _ := json.Marshal(map[string]any{"id": "ada", "document": map[string]any{"name": "ada", "age": 37}})
	call(t, ctx, routes["elasticsearch.document.create"], sess, map[string]string{"index": index}, nil, docBody)
	call(t, ctx, routes["elasticsearch.index.refresh"], sess, map[string]string{"index": index}, nil, nil)
	call(t, ctx, routes["elasticsearch.index.overview"], sess, map[string]string{"index": index}, nil, nil)
	call(t, ctx, routes["elasticsearch.mapping.read"], sess, map[string]string{"index": index}, nil, nil)
	call(t, ctx, routes["elasticsearch.settings.read"], sess, map[string]string{"index": index}, nil, nil)
	call(t, ctx, routes["elasticsearch.aliases.list"], sess, map[string]string{"index": index}, nil, nil)
	call(t, ctx, routes["elasticsearch.shards.list"], sess, map[string]string{"index": index}, nil, nil)

	docs := call(t, ctx, routes["elasticsearch.documents.list"], sess, map[string]string{"index": index}, url.Values{"limit": []string{"10"}}, nil)
	items := pageItems(docs)
	if len(items) != 1 || items[0]["_id"] != "ada" {
		t.Fatalf("expected indexed document, got %#v", items)
	}
	read := call(t, ctx, routes["elasticsearch.document.read"], sess, map[string]string{"index": index, "id": "ada"}, nil, nil).(map[string]any)
	if read["_id"] != "ada" {
		t.Fatalf("unexpected document read: %#v", read)
	}
	updateBody, _ := json.Marshal(map[string]any{"content": `{"name":"ada","age":38}`})
	call(t, ctx, routes["elasticsearch.document.update"], sess, map[string]string{"index": index, "id": "ada"}, nil, updateBody)
	call(t, ctx, routes["elasticsearch.document.delete"], sess, map[string]string{"index": index, "id": "ada"}, nil, nil)
}

func elasticsearchIntegrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	endpoint := os.Getenv("SHELLCN_ELASTICSEARCH_ENDPOINT")
	if endpoint == "" {
		endpoint = startElasticsearchContainer(ctx, t)
	}
	return map[string]any{"endpoint": endpoint, "auth": "none", "tls_mode": "disable", "read_only": false, "page_limit": 100, "timeout": "60s"}
}

func startElasticsearchContainer(ctx context.Context, t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_ELASTICSEARCH_ENDPOINT is not set")
	}
	name := "shellcn-elasticsearch-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"-e", "discovery.type=single-node",
		"-e", "xpack.security.enabled=false",
		"-e", "xpack.security.http.ssl.enabled=false",
		"-e", "xpack.security.autoconfiguration.enabled=false",
		"-e", "xpack.ml.enabled=false",
		"-e", "xpack.watcher.enabled=false",
		"-e", "ingest.geoip.downloader.enabled=false",
		"-e", "ES_JAVA_OPTS=-Xms512m -Xmx512m",
		"-p", "127.0.0.1::9200",
		"docker.elastic.co/elasticsearch/elasticsearch:8.15.5")
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "9200/tcp")
	host, port, err := net.SplitHostPort(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	endpoint := "http://" + net.JoinHostPort(host, port)
	cfg := map[string]any{"endpoint": endpoint, "auth": "none", "tls_mode": "disable", "read_only": false, "page_limit": 100, "timeout": "60s"}
	deadline := time.Now().Add(240 * time.Second)
	for {
		sess, err := escompat.Connect(ctx, plugin.ConnectConfig{Config: cfg, Net: transport.NewDirectForConnection(models.Connection{Config: cfg})}, escompat.Provider{Protocol: "elasticsearch", DefaultURL: endpoint, Product: escompat.ProductElasticsearch})
		if err == nil {
			_ = sess.Close()
			readyCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
			readyErr := elasticsearchReady(readyCtx, endpoint, ".shellcn-ready-"+time.Now().UTC().Format("20060102150405.000000000"))
			cancel()
			if readyErr == nil {
				return endpoint
			}
			err = readyErr
		}
		if time.Now().After(deadline) {
			t.Fatalf("Elasticsearch container did not become ready: %v", err)
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func elasticsearchReady(ctx context.Context, endpoint, index string) error {
	body := []byte(`{"settings":{"number_of_replicas":0}}`)
	if err := elasticsearchRaw(ctx, http.MethodPut, endpoint+"/"+index, body); err != nil {
		return err
	}
	deleteCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = elasticsearchRaw(deleteCtx, http.MethodDelete, endpoint+"/"+index, nil)
	return nil
}

func elasticsearchRaw(ctx context.Context, method, url string, body []byte) error {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s returned %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func routeMap(routes []plugin.Route) map[string]plugin.Route {
	out := map[string]plugin.Route{}
	for _, route := range routes {
		out[route.ID] = route
	}
	return out
}

func call(t *testing.T, ctx context.Context, route plugin.Route, sess plugin.Session, params map[string]string, query url.Values, body []byte) any {
	t.Helper()
	out, err := route.Handle(plugin.NewRequestContext(ctx, models.User{}, sess, params, query, body))
	if err != nil {
		t.Fatalf("%s: %v", route.ID, err)
	}
	return out
}

func callNoFail(ctx context.Context, route plugin.Route, sess plugin.Session, params map[string]string) {
	_, _ = route.Handle(plugin.NewRequestContext(ctx, models.User{}, sess, params, nil, nil))
}

func pageItems(page any) []map[string]any {
	data, _ := json.Marshal(page)
	var decoded struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(data, &decoded)
	return decoded.Items
}

func run(ctx context.Context, t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

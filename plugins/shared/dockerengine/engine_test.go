package dockerengine

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/moby/moby/api/types/events"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

func TestSortRowsOrdersNumericColumnsByValue(t *testing.T) {
	rows := []Row{{"name": "a", "size": int64(9)}, {"name": "b", "size": int64(1000)}, {"name": "c", "size": int64(200)}}
	sortRows(rows, []plugin.SortKey{{Field: "size"}})
	want := []any{int64(9), int64(200), int64(1000)}
	for i := range want {
		if rows[i]["size"] != want[i] {
			t.Fatalf("ascending size sort = %v %v %v, want %v", rows[0]["size"], rows[1]["size"], rows[2]["size"], want)
		}
	}
	sortRows(rows, []plugin.SortKey{{Field: "size", Desc: true}})
	if rows[0]["size"] != int64(1000) {
		t.Fatalf("descending size sort head = %v, want 1000", rows[0]["size"])
	}
}

func TestContainerEventKind(t *testing.T) {
	cases := []struct {
		action  events.Action
		evType  string
		state   string
		emitted bool
	}{
		{events.ActionCreate, "added", "created", true},
		{events.ActionDestroy, "deleted", "", true},
		{events.ActionDie, "updated", "exited", true},
		{events.ActionStop, "updated", "exited", true},
		{events.ActionStart, "updated", "running", true},
		{events.ActionExecStart, "", "", false},
		{events.ActionAttach, "", "", false},
	}
	for _, c := range cases {
		evType, state, ok := containerEventKind(c.action)
		if ok != c.emitted || evType != c.evType || state != c.state {
			t.Fatalf("%s => (%q,%q,%v), want (%q,%q,%v)", c.action, evType, state, ok, c.evType, c.state, c.emitted)
		}
	}
}

func TestResourceEventDieKeepsContainerListedAsExited(t *testing.T) {
	ev := resourceEventFromDocker(events.Message{
		Action: events.ActionDie,
		Actor:  events.Actor{ID: "abc123", Attributes: map[string]string{"name": "web"}},
	})
	if ev == nil || ev.Type != "updated" {
		t.Fatalf("die event = %+v, want updated", ev)
	}
	res := ev.Resource.(Row)
	if res["state"] != "exited" {
		t.Fatalf("die state = %v, want exited", res["state"])
	}
}

func TestRoutesAgainstFakeDockerDaemon(t *testing.T) {
	srv, calls := fakeDockerDaemon(t)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host, port, _ := net.SplitHostPort(u.Host)
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Config: map[string]any{"endpoint_type": "tcp", "host": host, "port": mustPort(t, port)},
		Net:    directNet{},
	}, "/var/run/docker.sock")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	rc := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, nil, url.Values{}, nil)
	got, err := ListContainers(rc)
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	page := got.(plugin.Page[Row])
	if len(page.Items) != 1 || page.Items[0]["name"] != "web" {
		t.Fatalf("container page unexpected: %+v", page.Items)
	}

	inspectRC := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, map[string]string{"id": "abc123"}, url.Values{}, nil)
	doc, err := InspectContainer(inspectRC)
	if err != nil {
		t.Fatalf("inspect container: %v", err)
	}
	asMap := doc.(map[string]any)
	if asMap["Name"] != "/web" {
		t.Fatalf("inspect name = %#v", asMap["Name"])
	}

	overview, err := ContainerOverview(inspectRC)
	if err != nil {
		t.Fatalf("container overview: %v", err)
	}
	if fmt.Sprint(overview.(Row)["name"]) != "web" || fmt.Sprint(overview.(Row)["state"]) != "running" {
		t.Fatalf("container overview unexpected: %+v", overview)
	}

	composeRC := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, map[string]string{"project": "demo"}, url.Values{}, nil)
	services, err := ComposeServices(composeRC)
	if err != nil {
		t.Fatalf("compose services: %v", err)
	}
	servicePage := services.(plugin.Page[Row])
	if len(servicePage.Items) != 1 || servicePage.Items[0]["name"] != "web" || servicePage.Items[0]["running"] != 1 {
		t.Fatalf("compose services unexpected: %+v", servicePage.Items)
	}

	if _, err := StartContainer(inspectRC); err != nil {
		t.Fatalf("start container: %v", err)
	}
	if !calls["POST /containers/abc123/start"] {
		t.Fatalf("start endpoint not called: %+v", calls)
	}

	createBody := `{"name":"api","image":"nginx:latest","pull":false,"start":true,"command":"nginx -g 'daemon off;'","env":"APP_ENV=test","ports":"8080:80/tcp","binds":"/srv/app:/app:ro","network":"bridge","restart":"unless-stopped"}`
	createRC := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, nil, url.Values{}, []byte(createBody))
	created, err := CreateContainer(createRC)
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	createResult := created.(CreateContainerResult)
	if !createResult.OK || createResult.ID != "def456789abc" || !createResult.Started {
		t.Fatalf("create result unexpected: %+v", createResult)
	}
	if !calls["POST /containers/create"] || !calls["POST /containers/def456789abcdef/start"] {
		t.Fatalf("create/start endpoints not called: %+v", calls)
	}

	body := `{"method":"GET","url":"/version","headers":[]}`
	apiRC := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, nil, url.Values{}, []byte(body))
	raw, err := ExecuteAPI(apiRC)
	if err != nil {
		t.Fatalf("execute api: %v", err)
	}
	resp := raw.(APIResponse)
	if resp.Status != http.StatusOK {
		t.Fatalf("raw api status = %d", resp.Status)
	}
}

type directNet struct{}

func (directNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func (directNet) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

func mustPort(t *testing.T, port string) int {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(port, "%d", &n); err != nil {
		t.Fatalf("parse port %q: %v", port, err)
	}
	return n
}

func fakeDockerDaemon(t *testing.T) (*httptest.Server, map[string]bool) {
	t.Helper()
	calls := map[string]bool{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := dockerAPIPath(r.URL.Path)
		calls[r.Method+" "+p] = true
		w.Header().Set("Content-Type", "application/json")
		switch {
		case p == "/_ping":
			w.Header().Set("Api-Version", "1.54")
			_, _ = w.Write([]byte("OK"))
		case p == "/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"Version": "28.5.2"})
		case p == "/containers/json":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"Id":      "abc123",
				"Names":   []string{"/web"},
				"Image":   "nginx:latest",
				"ImageID": "sha256:img",
				"Command": "nginx",
				"Created": float64(1710000000),
				"State":   "running",
				"Status":  "Up 2 minutes",
				"Labels":  map[string]string{"com.docker.compose.project": "demo", "com.docker.compose.service": "web"},
			}})
		case p == "/containers/abc123/json":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id":    "abc123",
				"Name":  "/web",
				"Image": "sha256:img",
				"Config": map[string]any{
					"Tty":    false,
					"Env":    []string{"APP_ENV=prod"},
					"Labels": map[string]string{"com.docker.compose.project": "demo", "com.docker.compose.service": "web"},
				},
				"State": map[string]any{"Status": "running", "Running": true},
			})
		case p == "/images/json":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"Id": "sha256:img", "RepoTags": []string{"nginx:latest"}, "Size": 1234, "Created": 1710000000, "Containers": 1}})
		case p == "/volumes":
			_ = json.NewEncoder(w).Encode(map[string]any{"Volumes": []map[string]any{{"Name": "data", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/data", "Scope": "local"}}})
		case p == "/networks":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"Id": "net1", "Name": "bridge", "Driver": "bridge", "Scope": "local"}})
		case r.Method == http.MethodPost && p == "/containers/abc123/start":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && p == "/containers/create":
			if got := r.URL.Query().Get("name"); got != "api" {
				t.Errorf("container create name query = %q, want api", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode create body: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if body["Image"] != "nginx:latest" {
				t.Errorf("create image = %#v", body["Image"])
			}
			env, _ := body["Env"].([]any)
			if len(env) != 1 || env[0] != "APP_ENV=test" {
				t.Errorf("create env = %#v", body["Env"])
			}
			cmd, _ := body["Cmd"].([]any)
			if len(cmd) != 3 || cmd[2] != "daemon off;" {
				t.Errorf("create command = %#v", body["Cmd"])
			}
			host, _ := body["HostConfig"].(map[string]any)
			if host["NetworkMode"] != "bridge" {
				t.Errorf("network mode = %#v", host["NetworkMode"])
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"Id": "def456789abcdef", "Warnings": []string{"created"}})
		case r.Method == http.MethodPost && p == "/containers/def456789abcdef/start":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Logf("unexpected docker request %s %s", r.Method, p)
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(h), calls
}

var versionPrefix = regexp.MustCompile(`^/v[0-9]+\.[0-9]+`)

func dockerAPIPath(path string) string {
	return versionPrefix.ReplaceAllString(path, "")
}

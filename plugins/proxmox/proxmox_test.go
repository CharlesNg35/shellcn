package proxmox

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestValidates(t *testing.T) {
	p := New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}

func TestNodeStorageTabIsNodeScoped(t *testing.T) {
	var storageTab *plugin.Panel
	for _, r := range New().Manifest().Resources {
		if r.Kind != "node" {
			continue
		}
		for i := range r.Detail.Tabs {
			if r.Detail.Tabs[i].Key == "storage" {
				storageTab = &r.Detail.Tabs[i]
			}
		}
	}
	if storageTab == nil || storageTab.Source == nil {
		t.Fatalf("node storage tab missing source")
	}
	if storageTab.Source.RouteID != "proxmox.node.storage" {
		t.Fatalf("node storage route = %q", storageTab.Source.RouteID)
	}
	if storageTab.Source.Params["node"] != "${resource.uid}" {
		t.Fatalf("node storage params = %+v", storageTab.Source.Params)
	}
}

func TestParseConnectOptions(t *testing.T) {
	cases := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
		check   func(connectOptions) bool
	}{
		{
			name: "token",
			cfg:  map[string]any{"host": "pve", "auth": "token", "token_id": "root@pam!ci", "token_secret": "s"},
			check: func(o connectOptions) bool {
				return o.Method == authToken && o.TokenID == "root@pam!ci" && o.Addr == "pve:8006"
			},
		},
		{
			name:  "password",
			cfg:   map[string]any{"host": "pve", "port": float64(443), "auth": "password", "username": "root@pam", "password": "p"},
			check: func(o connectOptions) bool { return o.Method == authPassword && o.Addr == "pve:443" },
		},
		{
			name: "credential",
			cfg: map[string]any{
				"host": "pve", "auth": "credential",
			},
			check: func(o connectOptions) bool { return o.Method == authToken && o.TokenID == "root@pam!stored" },
		},
		{name: "missing host", cfg: map[string]any{"auth": "token", "token_id": "x", "token_secret": "y"}, wantErr: true},
		{name: "token without secret", cfg: map[string]any{"host": "pve", "auth": "token", "token_id": "x"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := plugin.ConnectConfig{Config: tc.cfg}
			if tc.name == "credential" {
				cfg.Credentials = plugin.NewResolvedCredentials(plugin.CredentialBinding{
					Field: plugin.CredentialRefField,
					Credential: plugin.ResolvedCredential{Kind: CredentialProxmoxToken, Values: map[string]string{
						"token_id":     "root@pam!stored",
						"token_secret": "secret",
					}},
				})
			}
			opts, err := parseConnectOptions(cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.check(opts) {
				t.Fatalf("unexpected options: %+v", opts)
			}
		})
	}
}

func TestTermFraming(t *testing.T) {
	if got := string(inputFrame([]byte("ls\n"))); got != "0:3:ls\n" {
		t.Fatalf("inputFrame = %q", got)
	}
	if got := string(resizeFrame(80, 24)); got != "1:24:80:" {
		t.Fatalf("resizeFrame = %q (want rows:cols)", got)
	}
}

func TestMetricFrame(t *testing.T) {
	guest := metricFrame(plugin.TableRow{"cpu": 0.5, "cpus": float64(4), "mem": float64(512), "maxmem": float64(1024)})
	if guest["cpu"] != 50.0 || guest["cpuTotal"] != int64(4) || guest["mem"] != 50.0 || guest["memUsed"] != int64(512) || guest["memTotal"] != int64(1024) {
		t.Fatalf("guest metric = %+v", guest)
	}
	node := metricFrame(plugin.TableRow{"cpu": 0.1, "maxcpu": float64(8), "memory": map[string]any{"used": float64(1), "total": float64(4)}})
	if node["cpu"] != 10.0 || node["cpuTotal"] != int64(8) || node["mem"] != 25.0 || node["memUsed"] != int64(1) || node["memTotal"] != int64(4) {
		t.Fatalf("node metric = %+v", node)
	}
}

func TestRoutesAgainstFakeProxmox(t *testing.T) {
	srv := fakeProxmox(t)
	defer srv.Close()

	host, port := splitHostPort(t, srv.URL)
	sess := dialSession(t, host, port)

	t.Run("list qemu", func(t *testing.T) {
		page := callList(t, sess, listGuests("qemu"), nil)
		if len(page.Items) != 1 || page.Items[0]["name"] != "web" {
			t.Fatalf("qemu rows = %+v", page.Items)
		}
		if page.Items[0]["kindIcon"] != "monitor" || page.Items[0]["mode"] != "Template" {
			t.Fatalf("qemu presentation fields = %+v", page.Items[0])
		}
		if page.Items[0]["template"] != true {
			t.Fatalf("qemu template flag = %+v", page.Items[0]["template"])
		}
		ref := page.Items[0]["ref"].(plugin.ResourceRef)
		if ref.Namespace != "pve" || ref.UID != "100" {
			t.Fatalf("qemu ref = %+v", ref)
		}
	})

	t.Run("list lxc", func(t *testing.T) {
		page := callList(t, sess, listGuests("lxc"), nil)
		if len(page.Items) != 1 || page.Items[0]["name"] != "ct1" {
			t.Fatalf("lxc rows = %+v", page.Items)
		}
		if page.Items[0]["kindIcon"] != "box" || page.Items[0]["mode"] != "Instance" {
			t.Fatalf("lxc presentation fields = %+v", page.Items[0])
		}
	})

	t.Run("snapshot ref packs node/vmid/name", func(t *testing.T) {
		page := callList(t, sess, listSnapshots("qemu"), map[string]string{"node": "pve", "vmid": "100"})
		if len(page.Items) != 1 {
			t.Fatalf("snapshot rows = %+v", page.Items)
		}
		if page.Items[0]["vmstate"] != true {
			t.Fatalf("snapshot vmstate = %+v", page.Items[0]["vmstate"])
		}
		ref := page.Items[0]["ref"].(plugin.ResourceRef)
		if ref.Namespace != "pve" || ref.Name != "100" || ref.UID != "pre-upgrade" {
			t.Fatalf("snapshot ref = %+v", ref)
		}
	})

	t.Run("tree node children", func(t *testing.T) {
		result, err := treeNodes(newRC(sess, nil))
		if err != nil {
			t.Fatalf("tree: %v", err)
		}
		page := result.(plugin.Page[plugin.TreeNode])
		if len(page.Items) != 1 {
			t.Fatalf("expected one node, got %+v", page.Items)
		}
		if page.Items[0].ResourceKind != "guest" || page.Items[0].ChildrenSource != nil {
			t.Fatalf("node should open guest list without expanding: %+v", page.Items[0])
		}
		if page.Items[0].ListParams["node"] != "pve" {
			t.Fatalf("node list params = %+v", page.Items[0].ListParams)
		}
	})

	t.Run("list node guests", func(t *testing.T) {
		page := callList(t, sess, listGuests(""), map[string]string{"node": "pve"})
		if len(page.Items) != 2 {
			t.Fatalf("guest rows = %+v", page.Items)
		}
		var ref plugin.ResourceRef
		for _, item := range page.Items {
			candidate := item["ref"].(plugin.ResourceRef)
			if candidate.Kind == "qemu" {
				ref = candidate
				break
			}
		}
		if ref.Kind != "qemu" || ref.Namespace != "pve" || ref.UID != "100" {
			t.Fatalf("guest ref = %+v", ref)
		}
	})

	t.Run("list node storage", func(t *testing.T) {
		page := callList(t, sess, listNodeStorage, map[string]string{"node": "pve"})
		if len(page.Items) != 2 {
			t.Fatalf("storage rows = %+v", page.Items)
		}
		var local plugin.TableRow
		for _, item := range page.Items {
			if item["name"] == "local" {
				local = item
				break
			}
		}
		if local == nil {
			t.Fatalf("local storage row missing: %+v", page.Items)
		}
		if local["usedPct"] != 10.0 {
			t.Fatalf("storage usedPct = %+v", local["usedPct"])
		}
		ref := local["ref"].(plugin.ResourceRef)
		if ref.Kind != "storage" || ref.Namespace != "pve" || ref.UID != "local" {
			t.Fatalf("storage ref = %+v", ref)
		}
	})

	t.Run("backup storage options are node scoped", func(t *testing.T) {
		page := callList(t, sess, backupStorageOptions, map[string]string{"node": "pve"})
		if len(page.Items) != 1 || page.Items[0]["value"] != "local" {
			t.Fatalf("backup storage options = %+v", page.Items)
		}
	})
}

func TestTemplateUXContract(t *testing.T) {
	m := New().Manifest()
	actions := map[string]plugin.Action{}
	for _, action := range m.Actions {
		actions[action.ID] = action
	}
	for _, id := range []string{"act.qemu.start", "act.qemu.reboot", "act.qemu.migrate", "act.qemu.resize"} {
		if actions[id].VisibleWhen == nil {
			t.Fatalf("%s should be hidden for templates", id)
		}
	}
	if actions["act.qemu.clone"].VisibleWhen != nil {
		t.Fatalf("clone should remain visible for templates")
	}

	var qemu plugin.ResourceType
	for _, resource := range m.Resources {
		if resource.Kind == "qemu" {
			qemu = resource
			break
		}
	}
	if qemu.Kind == "" {
		t.Fatal("qemu resource missing")
	}
	for _, tab := range qemu.Detail.Tabs {
		switch tab.Key {
		case "metrics", "console", "snapshots":
			if tab.VisibleWhen == nil {
				t.Fatalf("%s tab should be hidden for templates", tab.Key)
			}
		case "summary", "hardware", "backups":
			if tab.VisibleWhen != nil {
				t.Fatalf("%s tab should remain visible for templates", tab.Key)
			}
		}
	}
}

func TestMigrateUsesNodeOptions(t *testing.T) {
	var field *plugin.Field
	for _, group := range migrateSchema().Groups {
		for i := range group.Fields {
			if group.Fields[i].Key == "target" {
				field = &group.Fields[i]
			}
		}
	}
	if field == nil {
		t.Fatal("target field missing")
	}
	if field.Type != plugin.FieldSelect {
		t.Fatalf("target field type = %q", field.Type)
	}
	if field.OptionsSource == nil || field.OptionsSource.RouteID != "proxmox.node.options" {
		t.Fatalf("target options source = %+v", field.OptionsSource)
	}
}

func TestBackupUXContract(t *testing.T) {
	m := New().Manifest()
	actions := map[string]plugin.Action{}
	for _, action := range m.Actions {
		actions[action.ID] = action
	}
	deleteBackup := actions["act.backup.delete"]
	if deleteBackup.Label != "Delete backup" || deleteBackup.VisibleWhen == nil {
		t.Fatalf("backup delete action = %+v", deleteBackup)
	}

	var qemu plugin.ResourceType
	var node plugin.ResourceType
	for _, resource := range m.Resources {
		switch resource.Kind {
		case "qemu":
			qemu = resource
		case "node":
			node = resource
		}
	}
	if qemu.Kind == "" || node.Kind == "" {
		t.Fatalf("missing resources: qemu=%q node=%q", qemu.Kind, node.Kind)
	}
	for _, tab := range qemu.Detail.Tabs {
		if tab.Key == "backups" {
			cfg := tab.Config.(plugin.TableConfig)
			if cfg.EmptyText == "" {
				t.Fatalf("backup table missing empty text")
			}
			if len(cfg.Columns) < 4 || cfg.Columns[3].Key != "protected" {
				t.Fatalf("backup columns = %+v", cfg.Columns)
			}
			if len(cfg.RowActionIDs) != 2 || cfg.RowActionIDs[0] != "act.qemu.backup.restore" || cfg.RowActionIDs[1] != "act.backup.delete" {
				t.Fatalf("backup row actions = %+v", cfg.RowActionIDs)
			}
		}
	}
	for _, tab := range node.Detail.Tabs {
		if tab.Key == "tasks" {
			cfg := tab.Config.(plugin.TableConfig)
			if tab.Label != "Task History" || cfg.DefaultSort == nil || cfg.DefaultSort.Field != "starttime" || !cfg.DefaultSort.Desc {
				t.Fatalf("task tab/config = %+v %+v", tab.Label, cfg.DefaultSort)
			}
		}
	}
}

func TestStorageColumnsExposeCapacityUsage(t *testing.T) {
	var usedPct *plugin.Column
	for _, column := range storageColumns() {
		if column.Key == "usedPct" {
			usedPct = &column
			break
		}
	}
	if usedPct == nil {
		t.Fatal("storage columns missing usedPct")
	}
	if usedPct.Type != plugin.ColumnPercent || usedPct.Precision == nil || *usedPct.Precision != 1 {
		t.Fatalf("storage usedPct column = %+v", usedPct)
	}
}

func TestBackupSchemaUsesStoragePicker(t *testing.T) {
	var field *plugin.Field
	for _, group := range backupSchema().Groups {
		for i := range group.Fields {
			if group.Fields[i].Key == "storage" {
				field = &group.Fields[i]
			}
		}
	}
	if field == nil {
		t.Fatal("storage field missing")
	}
	if field.Type != plugin.FieldSelect {
		t.Fatalf("storage field type = %q", field.Type)
	}
	if field.OptionsSource == nil || field.OptionsSource.RouteID != "proxmox.node.backup_storage.options" {
		t.Fatalf("storage options source = %+v", field.OptionsSource)
	}
}

func TestGuestOverviewMemorySemantics(t *testing.T) {
	srv := fakeProxmox(t)
	defer srv.Close()

	host, port := splitHostPort(t, srv.URL)
	sess := dialSession(t, host, port)
	result, err := guestOverview("qemu")(newRC(sess, map[string]string{"node": "pve", "vmid": "100"}))
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	got := result.(plugin.TableRow)
	if got["cpuTotal"] != int64(2) {
		t.Fatalf("cpuTotal = %+v, want sockets*cores", got["cpuTotal"])
	}
	if got["mem"] != int64(1073741824) || got["maxmem"] != int64(2147483648) || got["memPct"] != 50.0 {
		t.Fatalf("runtime memory fields = %+v", got)
	}
	if got["memoryConfigured"] != int64(2147483648) || got["memoryMinimum"] != int64(536870912) || got["memoryCurrent"] != int64(2147483648) {
		t.Fatalf("configured memory fields = %+v", got)
	}
}

func TestProxmoxUXInformationArchitecture(t *testing.T) {
	m := New().Manifest()
	byKind := map[string]plugin.ResourceType{}
	for _, resource := range m.Resources {
		byKind[resource.Kind] = resource
	}
	if byKind["task"].List.RouteID != "proxmox.task.list" {
		t.Fatalf("task resource list = %+v", byKind["task"].List)
	}
	for _, tab := range byKind["task"].Detail.Tabs {
		if tab.Key == "log" && tab.Type != plugin.PanelTable {
			t.Fatalf("task log tab = %+v", tab)
		}
	}
	for _, tab := range byKind["qemu"].Detail.Tabs {
		if tab.Key == "summary" {
			cfg := tab.Config.(plugin.ObjectDetailConfig)
			if !hasUsageField(cfg, "cpu") || !hasUsageField(cfg, "memPct") {
				t.Fatalf("qemu summary should use generic usage fields: %+v", cfg.Sections)
			}
		}
		if tab.Key == "metrics" {
			cfg := tab.Config.(plugin.MetricsConfig)
			if len(cfg.Gauges) != 0 || len(cfg.Usage) != 2 {
				t.Fatalf("qemu metrics should prefer usage rows over duplicate gauges: %+v", cfg)
			}
		}
		if tab.Key == "hardware" {
			cfg := tab.Config.(plugin.ObjectDetailConfig)
			if len(cfg.Sections) < 4 || cfg.Sections[2].Title != "Disks" || cfg.Sections[3].Title != "Network" {
				t.Fatalf("qemu hardware sections = %+v", cfg.Sections)
			}
		}
	}
}

func hasUsageField(cfg plugin.ObjectDetailConfig, key string) bool {
	for _, section := range cfg.Sections {
		for _, field := range section.Fields {
			if field.Key == key && field.Usage != nil {
				return true
			}
		}
	}
	return false
}

func TestGuestStoragePickerUsesContentContext(t *testing.T) {
	qemu := cloneSchema("qemu")
	lxc := cloneSchema("lxc")
	if got := schemaField(t, qemu, "storage").OptionsSource.Params["content"]; got != "images" {
		t.Fatalf("qemu clone storage content = %q", got)
	}
	if got := schemaField(t, lxc, "storage").OptionsSource.Params["content"]; got != "rootdir" {
		t.Fatalf("lxc clone storage content = %q", got)
	}
	restore := restoreSchema("qemu")
	archive := schemaField(t, restore, "archive")
	if archive.Default != "${resource.uid}" {
		t.Fatalf("restore archive default = %#v", archive.Default)
	}
}

func schemaField(t *testing.T, schema *plugin.Schema, key string) plugin.Field {
	t.Helper()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("field %q missing", key)
	return plugin.Field{}
}

// --- helpers --------------------------------------------------------------

func fakeProxmox(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/version", jsonHandler(`{"data":{"version":"8.1.0","release":"8"}}`))
	mux.HandleFunc("/api2/json/cluster/resources", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") == "storage" {
			_, _ = w.Write([]byte(`{"data":[{"type":"storage","storage":"local","node":"pve","plugintype":"dir","content":"backup,iso","disk":10,"maxdisk":100}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[
			{"type":"qemu","vmid":100,"name":"web","node":"pve","status":"running","template":1,"cpu":0.25,"mem":1073741824,"maxmem":2147483648,"uptime":3600},
			{"type":"lxc","vmid":200,"name":"ct1","node":"pve","status":"stopped","cpu":0,"mem":0,"maxmem":536870912}
			]}`))
	})
	mux.HandleFunc("/api2/json/nodes", jsonHandler(`{"data":[{"node":"pve","status":"online","cpu":0.1,"mem":1073741824,"maxmem":4294967296,"uptime":7200}]}`))
	mux.HandleFunc("/api2/json/nodes/pve/storage", jsonHandler(`{"data":[{"storage":"local","type":"dir","content":"backup,iso","used":10,"total":100,"active":1},{"storage":"local-lvm","type":"lvmthin","content":"images,rootdir","used":20,"total":200,"active":1}]}`))
	mux.HandleFunc("/api2/json/nodes/pve/qemu/100/status/current", jsonHandler(`{"data":{"status":"running","cpu":0.25,"mem":1073741824,"maxmem":2147483648,"uptime":3600}}`))
	mux.HandleFunc("/api2/json/nodes/pve/qemu/100/config", jsonHandler(`{"data":{"name":"web","memory":2048,"balloon":512,"cores":2,"sockets":1,"ostype":"l26","tags":"prod"}}`))
	mux.HandleFunc("/api2/json/nodes/pve/qemu/100/snapshot", jsonHandler(`{"data":[{"name":"pre-upgrade","description":"before update","snaptime":1700000000,"vmstate":1}]}`))
	srv := httptest.NewTLSServer(mux)
	return srv
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func dialSession(t *testing.T, host, port string) *Session {
	t.Helper()
	cfg := plugin.ConnectConfig{
		Net: directNet{},
		Config: map[string]any{
			"host": host, "port": atoi(port), "verify_tls": false,
			"auth": "token", "token_id": "root@pam!test", "token_secret": "secret",
		},
	}
	s, err := connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return s.(*Session)
}

func callList(t *testing.T, sess *Session, h plugin.Handler, params map[string]string) plugin.Page[plugin.TableRow] {
	t.Helper()
	result, err := h(newRC(sess, params))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	page, ok := result.(plugin.Page[plugin.TableRow])
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	return page
}

func newRC(sess *Session, params map[string]string) *plugin.RequestContext {
	return plugin.NewRequestContext(context.Background(), plugin.User{}, sess, params, url.Values{}, nil)
}

// directNet dials the literal address — the fake server runs on the configured
// host:port, so the same transport the gateway wires is exercised end to end.
type directNet struct{}

func (directNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, addr)
}

func (directNet) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

func splitHostPort(t *testing.T, raw string) (host, port string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	host, port, err = net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host: %v", err)
	}
	return host, port
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

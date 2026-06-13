package proxmox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestValidators(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) bool
		ok   []string
		bad  []string
	}{
		{"vmid", validVMID, []string{"100", "9", "999999999"}, []string{"", "0", "01", "abc", "-1", "10.5", "12345678901"}},
		{"node", validNode, []string{"pve", "pve-1", "node.example", "n0"}, []string{"", "-bad", "../etc", "a b", "no/slash"}},
		{"disk", validDisk, []string{"scsi0", "virtio1", "sata15", "ide0"}, []string{"", "scsi", "0scsi", "scsi-0", "SCSI0"}},
		{"size", validSize, []string{"50G", "+10G", "100", "8M", "2T", "512K"}, []string{"", "G", "+", "0G", "-5G", "10GB", "10g"}},
		{"power", validPowerCommand, []string{"reboot", "shutdown"}, []string{"", "stop", "start", "Reboot", "poweroff"}},
		{"backup volume", func(s string) bool { return validBackupVolume("local", s) }, []string{"local:backup/vzdump-qemu-100.vma.zst"}, []string{"", "other:backup/vzdump-qemu-100.vma.zst", "local:iso/debian.iso", "local:backup/file?v=1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, s := range tc.ok {
				if !tc.fn(s) {
					t.Errorf("%q should be valid", s)
				}
			}
			for _, s := range tc.bad {
				if tc.fn(s) {
					t.Errorf("%q should be invalid", s)
				}
			}
		})
	}
}

func TestValidUPID(t *testing.T) {
	ok := []string{
		"UPID:pve:00001234:0AB12345:65000000:qmclone:100:root@pam:",
		"UPID:pve-1:000005DC:00ABCDEF:66000000:vzdump::root@pam:",
		"UPID:node.dc:00000001:00000001:00000001:qmstart:101:user@pve!token:",
	}
	bad := []string{
		"",
		"not-a-upid",
		"UPID:pve:00001234:0AB12345:65000000:qmclone:100:root@pam", // missing trailing colon
		"UPID:pve:zz:0AB12345:65000000:qmclone:100:root@pam:",      // non-hex pid
	}
	for _, s := range ok {
		if !validUPID(s) {
			t.Errorf("%q should be a valid UPID", s)
		}
	}
	for _, s := range bad {
		if validUPID(s) {
			t.Errorf("%q should be an invalid UPID", s)
		}
	}
}

func TestCloneBody(t *testing.T) {
	t.Run("qemu maps name and flags", func(t *testing.T) {
		b, err := cloneBody("qemu", "101", "clone-vm", "pve2", "local-lvm", true)
		if err != nil {
			t.Fatal(err)
		}
		if b["newid"] != "101" || b["name"] != "clone-vm" || b["target"] != "pve2" || b["storage"] != "local-lvm" || b["full"] != 1 {
			t.Fatalf("body = %+v", b)
		}
		if _, ok := b["hostname"]; ok {
			t.Fatalf("qemu must not set hostname: %+v", b)
		}
	})
	t.Run("lxc maps hostname", func(t *testing.T) {
		b, err := cloneBody("lxc", "201", "ct-clone", "", "", false)
		if err != nil {
			t.Fatal(err)
		}
		if b["hostname"] != "ct-clone" {
			t.Fatalf("lxc should set hostname: %+v", b)
		}
		if _, ok := b["full"]; ok {
			t.Fatalf("linked clone must not set full: %+v", b)
		}
	})
	t.Run("rejects bad newid", func(t *testing.T) {
		if _, err := cloneBody("qemu", "abc", "", "", "", false); err == nil {
			t.Fatal("expected error for bad newid")
		}
	})
	t.Run("rejects bad target", func(t *testing.T) {
		if _, err := cloneBody("qemu", "101", "", "bad/node", "", false); err == nil {
			t.Fatal("expected error for bad target")
		}
	})
}

func TestRestoreBody(t *testing.T) {
	t.Run("qemu uses archive", func(t *testing.T) {
		b, err := restoreBody("qemu", "300", "local:backup/vzdump-qemu-100.vma.zst", "local-lvm", true)
		if err != nil {
			t.Fatal(err)
		}
		if b["archive"] == nil || b["force"] != 1 || b["storage"] != "local-lvm" {
			t.Fatalf("body = %+v", b)
		}
		if _, ok := b["restore"]; ok {
			t.Fatalf("qemu must not set restore flag: %+v", b)
		}
	})
	t.Run("lxc uses ostemplate + restore", func(t *testing.T) {
		b, err := restoreBody("lxc", "301", "local:backup/vzdump-lxc-200.tar.zst", "", false)
		if err != nil {
			t.Fatal(err)
		}
		if b["ostemplate"] == nil || b["restore"] != 1 {
			t.Fatalf("body = %+v", b)
		}
	})
	t.Run("requires archive", func(t *testing.T) {
		if _, err := restoreBody("qemu", "300", "", "", false); err == nil {
			t.Fatal("expected error for missing archive")
		}
	})
	t.Run("rejects bad vmid", func(t *testing.T) {
		if _, err := restoreBody("qemu", "0", "x", "", false); err == nil {
			t.Fatal("expected error for bad vmid")
		}
	})
}

func TestPVEPathEscapesDynamicSegments(t *testing.T) {
	got := pvePath("nodes", "pve", "storage", "local", "content", "local:backup/vzdump-qemu-100.vma.zst")
	want := "/nodes/pve/storage/local/content/local:backup%2Fvzdump-qemu-100.vma.zst"
	if got != want {
		t.Fatalf("pvePath = %q, want %q", got, want)
	}
}

// --- httptest handler coverage --------------------------------------------

func TestOpsAgainstFakeProxmox(t *testing.T) {
	calls := &recorder{posts: map[string]json.RawMessage{}, puts: map[string]json.RawMessage{}, deletes: map[string]bool{}}
	srv := fakeProxmoxOps(t, calls)
	defer srv.Close()

	host, port := splitHostPort(t, srv.URL)
	sess := dialSession(t, host, port)

	t.Run("qemu clone returns upid", func(t *testing.T) {
		res, err := guestClone("qemu")(rcWithBody(sess, map[string]string{"node": "pve", "vmid": "100"},
			`{"newid":101,"name":"web2","full":true}`))
		if err != nil {
			t.Fatal(err)
		}
		tr := res.(taskResult)
		if !tr.OK || tr.UPID == "" {
			t.Fatalf("clone result = %+v", tr)
		}
		body := calls.posts["/api2/json/nodes/pve/qemu/100/clone"]
		if body == nil {
			t.Fatal("clone POST not received")
		}
		var got map[string]any
		_ = json.Unmarshal(body, &got)
		if got["newid"] != "101" || got["name"] != "web2" || got["full"] != float64(1) {
			t.Fatalf("clone body = %s", body)
		}
	})

	t.Run("lxc clone sets hostname", func(t *testing.T) {
		if _, err := guestClone("lxc")(rcWithBody(sess, map[string]string{"node": "pve", "vmid": "200"},
			`{"newid":"201","name":"ct2"}`)); err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		_ = json.Unmarshal(calls.posts["/api2/json/nodes/pve/lxc/200/clone"], &got)
		if got["hostname"] != "ct2" {
			t.Fatalf("lxc clone body = %s", calls.posts["/api2/json/nodes/pve/lxc/200/clone"])
		}
	})

	t.Run("qemu resize sends PUT", func(t *testing.T) {
		res, err := qemuResize(rcWithBody(sess, map[string]string{"node": "pve", "vmid": "100"},
			`{"disk":"scsi0","size":"+10G"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !res.(taskResult).OK {
			t.Fatalf("resize result = %+v", res)
		}
		var got map[string]any
		_ = json.Unmarshal(calls.puts["/api2/json/nodes/pve/qemu/100/resize"], &got)
		if got["disk"] != "scsi0" || got["size"] != "+10G" {
			t.Fatalf("resize body = %s", calls.puts["/api2/json/nodes/pve/qemu/100/resize"])
		}
	})

	t.Run("qemu resize rejects bad size", func(t *testing.T) {
		if _, err := qemuResize(rcWithBody(sess, map[string]string{"node": "pve", "vmid": "100"},
			`{"disk":"scsi0","size":"big"}`)); err == nil {
			t.Fatal("expected error for bad size")
		}
	})

	t.Run("qemu destroy purges", func(t *testing.T) {
		res, err := guestDestroy("qemu")(rcWithBody(sess, map[string]string{"node": "pve", "vmid": "100"}, ""))
		if err != nil {
			t.Fatal(err)
		}
		if !res.(taskResult).OK {
			t.Fatalf("destroy result = %+v", res)
		}
		if !calls.deletes["/api2/json/nodes/pve/qemu/100"] {
			t.Fatalf("destroy DELETE not received: %+v", calls.deletes)
		}
	})

	t.Run("qemu restore creates from archive", func(t *testing.T) {
		if _, err := guestRestore("qemu")(rcWithBody(sess, map[string]string{"node": "pve"},
			`{"vmid":300,"archive":"local:backup/vzdump-qemu-100.vma.zst"}`)); err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		_ = json.Unmarshal(calls.posts["/api2/json/nodes/pve/qemu"], &got)
		if got["vmid"] != "300" || got["archive"] == nil {
			t.Fatalf("restore body = %s", calls.posts["/api2/json/nodes/pve/qemu"])
		}
	})

	t.Run("node power reboot", func(t *testing.T) {
		res, err := nodePower(rcWithBody(sess, map[string]string{"node": "pve"}, `{"command":"reboot"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !res.(actionResult).OK {
			t.Fatalf("power result = %+v", res)
		}
		var got map[string]any
		_ = json.Unmarshal(calls.posts["/api2/json/nodes/pve/status"], &got)
		if got["command"] != "reboot" {
			t.Fatalf("power body = %s", calls.posts["/api2/json/nodes/pve/status"])
		}
	})

	t.Run("backup create rejects invalid mode", func(t *testing.T) {
		if _, err := backupCreate(rcWithBody(sess, map[string]string{"node": "pve", "vmid": "100"},
			`{"storage":"local","mode":"invalid","compress":"zstd"}`)); err == nil {
			t.Fatal("expected error for invalid backup mode")
		}
	})

	t.Run("backup delete rejects non backup volume", func(t *testing.T) {
		if _, err := backupDelete(rcWithBody(sess, map[string]string{"node": "pve", "storage": "local", "volume": "local:iso/debian.iso"}, "")); err == nil {
			t.Fatal("expected error for non-backup volume")
		}
	})

	t.Run("node power rejects bad command", func(t *testing.T) {
		if _, err := nodePower(rcWithBody(sess, map[string]string{"node": "pve"}, `{"command":"explode"}`)); err == nil {
			t.Fatal("expected error for bad command")
		}
	})

	t.Run("task stop deletes", func(t *testing.T) {
		upid := "UPID:pve:00001234:0AB12345:65000000:qmclone:100:root@pam:"
		if _, err := taskStop(rcWithBody(sess, map[string]string{"node": "pve", "upid": upid}, "")); err != nil {
			t.Fatal(err)
		}
		if !calls.deletes["/api2/json/nodes/pve/tasks/"+upid] {
			t.Fatalf("task DELETE not received: %+v", calls.deletes)
		}
	})

	t.Run("task stop rejects bad upid", func(t *testing.T) {
		if _, err := taskStop(rcWithBody(sess, map[string]string{"node": "pve", "upid": "garbage"}, "")); err == nil {
			t.Fatal("expected error for bad upid")
		}
	})

	t.Run("task status reads object", func(t *testing.T) {
		upid := "UPID:pve:00001234:0AB12345:65000000:qmclone:100:root@pam:"
		res, err := taskStatus(rcWithBody(sess, map[string]string{"node": "pve", "upid": upid}, ""))
		if err != nil {
			t.Fatal(err)
		}
		obj := res.(row)
		if obj["status"] != "stopped" || obj["exitstatus"] != "OK" {
			t.Fatalf("task status = %+v", obj)
		}
	})

	t.Run("task log reads lines", func(t *testing.T) {
		upid := "UPID:pve:00001234:0AB12345:65000000:qmclone:100:root@pam:"
		res, err := taskLog(rcWithBody(sess, map[string]string{"node": "pve", "upid": upid}, ""))
		if err != nil {
			t.Fatal(err)
		}
		page := res.(plugin.Page[row])
		if len(page.Items) != 2 || page.Items[0]["t"] != "starting" {
			t.Fatalf("task log = %+v", page.Items)
		}
	})
}

// --- helpers --------------------------------------------------------------

type recorder struct {
	posts   map[string]json.RawMessage
	puts    map[string]json.RawMessage
	deletes map[string]bool
}

func fakeProxmoxOps(t *testing.T, calls *recorder) *httptest.Server {
	t.Helper()
	const upid = `"UPID:pve:00001234:0AB12345:65000000:qmclone:100:root@pam:"`
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/version", jsonHandler(`{"data":{"version":"8.1.0","release":"8"}}`))

	record := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.Method {
		case http.MethodPost:
			calls.posts[r.URL.Path] = json.RawMessage(body)
		case http.MethodPut:
			calls.puts[r.URL.Path] = json.RawMessage(body)
		case http.MethodDelete:
			calls.deletes[r.URL.Path] = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":` + upid + `}`))
	}

	for _, p := range []string{
		"/api2/json/nodes/pve/qemu/100/clone",
		"/api2/json/nodes/pve/lxc/200/clone",
		"/api2/json/nodes/pve/qemu/100/resize",
		"/api2/json/nodes/pve/qemu/100",
		"/api2/json/nodes/pve/qemu",
		"/api2/json/nodes/pve/lxc",
		"/api2/json/nodes/pve/status",
	} {
		mux.HandleFunc(p, record)
	}
	// Task endpoints share a prefix; route by suffix.
	mux.HandleFunc("/api2/json/nodes/pve/tasks/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			calls.deletes[r.URL.Path] = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":null}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK","type":"qmclone","upid":"x"}}`))
		case strings.HasSuffix(r.URL.Path, "/log"):
			_, _ = w.Write([]byte(`{"data":[{"n":1,"t":"starting"},{"n":2,"t":"done"}]}`))
		default:
			http.NotFound(w, r)
		}
	})

	return httptest.NewTLSServer(mux)
}

func rcWithBody(sess *Session, params map[string]string, body string) *plugin.RequestContext {
	var b []byte
	if body != "" {
		b = []byte(body)
	}
	return plugin.NewRequestContext(context.Background(), plugin.User{}, sess, params, url.Values{}, b)
}

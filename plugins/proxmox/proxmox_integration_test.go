package proxmox

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

// TestProxmoxPluginIntegration drives the new lifecycle handlers against a real
// Proxmox VE endpoint. Proxmox VE is a bare-metal/KVM hypervisor OS and CANNOT be
// self-provisioned via Docker, so this test is gated on a live endpoint supplied
// through the environment and skips otherwise.
//
// Required env (all read the same connection params as the plugin's config):
//
//	SHELLCN_PROXMOX_INTEGRATION=1     enable the test
//	SHELLCN_PROXMOX_HOST              PVE host (e.g. pve.example.com)
//	SHELLCN_PROXMOX_PORT              API port (default 8006)
//	SHELLCN_PROXMOX_TOKEN_ID          API token id (user@realm!name)
//	SHELLCN_PROXMOX_TOKEN_SECRET      API token secret
//	SHELLCN_PROXMOX_NODE              node hosting the template
//	SHELLCN_PROXMOX_TEMPLATE_VMID     a small qemu template to clone from
//	SHELLCN_PROXMOX_CLONE_VMID        an unused VMID to clone into (destroyed at end)
//
// Optional:
//
//	SHELLCN_PROXMOX_VERIFY_TLS=1      verify the server certificate (default off)
//	SHELLCN_PROXMOX_STORAGE          storage for the full clone
//	SHELLCN_PROXMOX_DISK             disk to resize (default scsi0)
//
// The test clones the template into a new VM, resizes a disk, reads the spawned
// task's status, then destroys the clone — a full round-trip on a throwaway guest.
func TestProxmoxPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_PROXMOX_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_PROXMOX_INTEGRATION=1 (and SHELLCN_PROXMOX_* connection vars) to run against a live Proxmox VE endpoint")
	}

	host := mustEnv(t, "SHELLCN_PROXMOX_HOST")
	tokenID := mustEnv(t, "SHELLCN_PROXMOX_TOKEN_ID")
	tokenSecret := mustEnv(t, "SHELLCN_PROXMOX_TOKEN_SECRET")
	node := mustEnv(t, "SHELLCN_PROXMOX_NODE")
	templateVMID := mustEnv(t, "SHELLCN_PROXMOX_TEMPLATE_VMID")
	cloneVMID := mustEnv(t, "SHELLCN_PROXMOX_CLONE_VMID")

	port := 8006
	if v := os.Getenv("SHELLCN_PROXMOX_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("SHELLCN_PROXMOX_PORT: %v", err)
		}
		port = p
	}
	disk := os.Getenv("SHELLCN_PROXMOX_DISK")
	if disk == "" {
		disk = "scsi0"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	sess, err := connect(ctx, plugin.ConnectConfig{
		Net: directNet{},
		Config: map[string]any{
			"host":         host,
			"port":         port,
			"verify_tls":   os.Getenv("SHELLCN_PROXMOX_VERIFY_TLS") == "1",
			"auth":         "token",
			"token_id":     tokenID,
			"token_secret": tokenSecret,
		},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	s := sess.(*Session)
	defer func() { _ = s.Close() }()

	call := func(handler plugin.Handler, params map[string]string, body string) any {
		t.Helper()
		var b []byte
		if body != "" {
			b = []byte(body)
		}
		rc := plugin.NewRequestContext(ctx, models.User{ID: "it"}, s, params, nil, b)
		res, err := handler(rc)
		if err != nil {
			t.Fatalf("handler: %v", err)
		}
		return res
	}

	// Ensure the clone is removed even if a later step fails.
	t.Cleanup(func() {
		cleanupCtx, cc := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cc()
		_, _ = s.delUPID(cleanupCtx, fmt.Sprintf("/nodes/%s/qemu/%s?purge=1&destroy-unreferenced-disks=1", node, cloneVMID))
	})

	// Clone the template into the throwaway VM and wait for the task to finish.
	cloneRes := call(guestClone("qemu"),
		map[string]string{"node": node, "vmid": templateVMID},
		fmt.Sprintf(`{"newid":%q,"name":"shellcn-it","storage":%q,"full":true}`, cloneVMID, os.Getenv("SHELLCN_PROXMOX_STORAGE")))
	cloneTask := cloneRes.(taskResult)
	if !cloneTask.OK || cloneTask.UPID == "" {
		t.Fatalf("clone returned no UPID: %+v", cloneTask)
	}
	waitTask(ctx, t, s, node, cloneTask.UPID)

	// Read the clone task's status through the handler.
	statusRes := call(taskStatus, map[string]string{"node": node, "upid": cloneTask.UPID}, "")
	if st := str(statusRes.(row)["status"]); st != "stopped" {
		t.Fatalf("clone task status = %q, want stopped", st)
	}

	// Resize a disk on the freshly cloned VM (relative grow).
	resizeRes := call(qemuResize,
		map[string]string{"node": node, "vmid": cloneVMID},
		fmt.Sprintf(`{"disk":%q,"size":"+1G"}`, disk))
	if rt := resizeRes.(taskResult); !rt.OK {
		t.Fatalf("resize result = %+v", rt)
	}

	// Destroy the clone and wait for completion.
	destroyRes := call(guestDestroy("qemu"), map[string]string{"node": node, "vmid": cloneVMID}, "")
	destroyTask := destroyRes.(taskResult)
	if !destroyTask.OK || destroyTask.UPID == "" {
		t.Fatalf("destroy returned no UPID: %+v", destroyTask)
	}
	waitTask(ctx, t, s, node, destroyTask.UPID)
}

// waitTask polls a UPID's status until it leaves the running state or the context
// deadline is hit.
func waitTask(ctx context.Context, t *testing.T, s *Session, node, upid string) {
	t.Helper()
	for {
		obj, err := s.object(ctx, fmt.Sprintf("/nodes/%s/tasks/%s/status", node, upid))
		if err != nil {
			t.Fatalf("task status %s: %v", upid, err)
		}
		if str(obj["status"]) != "running" {
			if exit := str(obj["exitstatus"]); exit != "" && exit != "OK" {
				t.Fatalf("task %s finished with exit status %q", upid, exit)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("task %s did not finish before deadline", upid)
		case <-time.After(2 * time.Second):
		}
	}
}

func mustEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("%s not set; skipping live Proxmox integration test", key)
	}
	return v
}

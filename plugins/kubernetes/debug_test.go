package kubernetes

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestPodDebugWired(t *testing.T) {
	route := false
	for _, r := range Routes() {
		if r.ID == "kubernetes.pod.debug.create" {
			if r.Method != plugin.MethodPost || r.Handle == nil {
				t.Fatalf("debug.create route = %+v", r)
			}
			route = true
		}
	}
	if !route {
		t.Fatal("kubernetes.pod.debug.create route missing")
	}

	// The Debug action creates the container, then opens an exec terminal via the
	// open_panel effect passing ${response.container}.
	a := debugAction()
	if a.RouteID != "kubernetes.pod.debug.create" {
		t.Fatalf("debug action route = %q", a.RouteID)
	}
	effects := a.OnSuccess.Effects
	if len(effects) != 1 || effects[0].Type != plugin.ActionEffectOpenPanel {
		t.Fatalf("debug action onSuccess = %+v", effects)
	}
	op := effects[0].OpenPanel
	if op == nil || op.Panel != plugin.PanelTerminal || op.Source == nil ||
		op.Source.RouteID != "kubernetes.pod.exec" ||
		op.Source.Params["container"] != "${response.container}" {
		t.Fatalf("debug open_panel = %+v", op)
	}

	pod, _ := kindByName("pod")
	found := false
	for _, id := range resourceType(pod).Actions.Detail {
		if id == "kubernetes.pod.debug" {
			found = true
		}
	}
	if !found {
		t.Fatal("pod detail actions must include kubernetes.pod.debug")
	}
}

func TestAddEphemeralContainer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": obj{"name": "web", "namespace": "default"},
		})
	})
	hit := false
	mux.HandleFunc("/api/v1/namespaces/default/pods/web/ephemeralcontainers", func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": obj{"name": "web", "namespace": "default"},
		})
	})
	sess := connectTo(t, mux).(*Session)

	name, err := sess.addEphemeralContainer(context.Background(), "default", "web", "busybox:1.36", "")
	if err != nil {
		t.Fatalf("add ephemeral container: %v", err)
	}
	if !strings.HasPrefix(name, "debugger-") {
		t.Fatalf("debug container name = %q, want debugger-*", name)
	}
	if !hit {
		t.Fatal("ephemeralcontainers subresource was not updated")
	}
}

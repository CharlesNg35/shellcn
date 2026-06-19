package kubernetes

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestResourceOverviewIncludesRBAC(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": obj{"name": "web", "namespace": "default"},
		})
	})
	// client-go posts protobuf; the verb is still legible in the raw body.
	mux.HandleFunc("/apis/authorization.k8s.io/v1/selfsubjectaccessreviews", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		writeJSON(w, obj{
			"apiVersion": "authorization.k8s.io/v1", "kind": "SelfSubjectAccessReview",
			"status": obj{"allowed": !strings.Contains(string(body), "delete")},
		})
	})
	sess := connectTo(t, mux)

	out, err := ResourceOverview(rc(sess, map[string]string{"kind": "pod", "namespace": "default", "name": "web"}))
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	row := out.(Row)
	if row["canDelete"] != false {
		t.Fatalf("canDelete = %v, want false", row["canDelete"])
	}
	if row["canPatch"] != true || row["canUpdate"] != true {
		t.Fatalf("canPatch=%v canUpdate=%v, want true", row["canPatch"], row["canUpdate"])
	}
}

func TestDestructiveActionsGatedByRBAC(t *testing.T) {
	want := map[string]string{
		"kubernetes.resource.delete":       "canDelete",
		"kubernetes.customresource.delete": "canDelete",
		"kubernetes.resource.scale":        "canPatch",
		"kubernetes.resource.restart":      "canPatch",
		"kubernetes.node.drain":            "canPatch",
		"kubernetes.rollout.undo":          "canPatch",
	}
	for _, a := range actions() {
		field, ok := want[a.ID]
		if !ok {
			continue
		}
		if a.EnabledWhen == nil || !hasRuleField(a.EnabledWhen.AllOf, field) {
			t.Errorf("action %s must be gated by %s", a.ID, field)
		}
		delete(want, a.ID)
	}
	if len(want) != 0 {
		t.Fatalf("gated actions missing: %v", want)
	}
}

func hasRuleField(rules []plugin.Rule, field string) bool {
	for _, r := range rules {
		if r.Field == field {
			return true
		}
	}
	return false
}

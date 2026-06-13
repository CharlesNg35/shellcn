package kubernetes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// resolveKind resolves the {kind} route param to a catalog kind or a runtime CRD.
func resolveKind(s *Session, name string) (kind, error) {
	if k, ok := kindByName(name); ok {
		return k, nil
	}
	return crdKind(s, name)
}

// listNamespace is the namespace a list/get scopes to: an explicit param wins,
// then the connection default, else all namespaces ("").
func (s *Session) listNamespace(rc *plugin.RequestContext, k kind) string {
	if !k.namespaced {
		return ""
	}
	if ns := rc.Param("namespace"); ns != "" {
		return ns
	}
	return s.defaultNS
}

func mapList(k kind, list *unstructured.UnstructuredList) []Row {
	rows := make([]Row, 0, len(list.Items))
	for i := range list.Items {
		o := list.Items[i].Object
		row := commonRow(o)
		if k.extra != nil {
			for key, val := range k.extra(o) {
				row[key] = val
			}
		}
		row["ref"] = rowRef(k, o)
		rows = append(rows, row)
	}
	return rows
}

// rowRef is the navigation/action identity the generic table reads from each
// row. CRD rows resolve to the generic customresource type with the concrete
// GVR carried in scope.
func rowRef(k kind, o obj) plugin.ResourceRef {
	ref := plugin.ResourceRef{Kind: k.name, Namespace: refNS(o), Name: refName(o), UID: str(o, "metadata", "uid")}
	if strings.HasPrefix(k.name, crdParamPrefix) {
		ref.Kind = customResourceKind
		ref.Scope = k.name
	}
	return ref
}

// ListResource lists any kind via the dynamic client (built-in GVR or CRD).
func ListResource(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	k, err := resolveKind(s, rc.Param("kind"))
	if err != nil {
		return nil, err
	}
	// CRDs render their own server-side printer columns via the Table API.
	if isCRD(k) {
		_, rows, err := s.tableList(rc.Ctx, k, s.listNamespace(rc, k), 0)
		if err != nil {
			return nil, err
		}
		return pageRows(rc, rows)
	}
	ri := s.Dynamic().Resource(k.gvr)
	opts := metav1.ListOptions{Limit: k.listLimit}
	var list *unstructured.UnstructuredList
	if ns := s.listNamespace(rc, k); k.namespaced && ns != "" {
		list, err = ri.Namespace(ns).List(rc.Ctx, opts)
	} else {
		list, err = ri.List(rc.Ctx, opts)
	}
	if err != nil {
		return nil, apiErr(err)
	}
	rows := mapList(k, list)
	if k.recentFirst {
		sortRecentFirst(rows)
	}
	return pageRows(rc, rows)
}

// sortRecentFirst orders rows newest-first; k8s RFC3339 UTC stamps sort
// chronologically as plain strings, so no parsing is needed.
func sortRecentFirst(rows []Row) {
	sort.Slice(rows, func(i, j int) bool {
		return fmt.Sprint(rows[i]["createdAt"]) > fmt.Sprint(rows[j]["createdAt"])
	})
}

// GetResource returns one object for the detail/YAML view (Secrets redacted).
func GetResource(rc *plugin.RequestContext) (any, error) {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return nil, err
	}
	o, err := s.get(rc, k, name)
	if err != nil {
		return nil, apiErr(err)
	}
	unstructured.RemoveNestedField(o.Object, "metadata", "managedFields")
	if k.redact {
		unstructured.RemoveNestedField(o.Object, "data")
		unstructured.RemoveNestedField(o.Object, "stringData")
	}
	return o.Object, nil
}

func (s *Session) get(rc *plugin.RequestContext, k kind, name string) (*unstructured.Unstructured, error) {
	ri := s.Dynamic().Resource(k.gvr)
	if ns := rc.Param("namespace"); k.namespaced && ns != "" {
		return ri.Namespace(ns).Get(rc.Ctx, name, metav1.GetOptions{})
	}
	return ri.Get(rc.Ctx, name, metav1.GetOptions{})
}

func DeleteResource(rc *plugin.RequestContext) (any, error) {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return nil, err
	}
	ri := s.Dynamic().Resource(k.gvr)
	if ns := rc.Param("namespace"); k.namespaced && ns != "" {
		err = ri.Namespace(ns).Delete(rc.Ctx, name, metav1.DeleteOptions{})
	} else {
		err = ri.Delete(rc.Ctx, name, metav1.DeleteOptions{})
	}
	if err != nil {
		return nil, apiErr(err)
	}
	return okResult(), nil
}

type ScaleRequest struct {
	Replicas int64 `json:"replicas"`
}

// ScaleResource sets spec.replicas on scalable workloads.
func ScaleResource(rc *plugin.RequestContext) (any, error) {
	var req ScaleRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	if req.Replicas < 0 {
		return nil, fmt.Errorf("%w: replicas must be >= 0", plugin.ErrInvalidInput)
	}
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, req.Replicas))
	return patchResource(rc, types.MergePatchType, patch)
}

// RestartResource triggers a rolling restart (the kubectl rollout-restart stamp).
func RestartResource(rc *plugin.RequestContext) (any, error) {
	stamp := time.Now().UTC().Format(time.RFC3339)
	patch := []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`, stamp))
	return patchResource(rc, types.StrategicMergePatchType, patch)
}

// CordonNode / UncordonNode toggle a node's schedulability.
func CordonNode(rc *plugin.RequestContext) (any, error)   { return setUnschedulable(rc, true) }
func UncordonNode(rc *plugin.RequestContext) (any, error) { return setUnschedulable(rc, false) }

func setUnschedulable(rc *plugin.RequestContext, v bool) (any, error) {
	patch := []byte(fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, v))
	return patchResource(rc, types.MergePatchType, patch)
}

func patchResource(rc *plugin.RequestContext, pt types.PatchType, patch []byte) (any, error) {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return nil, err
	}
	ri := s.Dynamic().Resource(k.gvr)
	if ns := rc.Param("namespace"); k.namespaced && ns != "" {
		_, err = ri.Namespace(ns).Patch(rc.Ctx, name, pt, patch, metav1.PatchOptions{})
	} else {
		_, err = ri.Patch(rc.Ctx, name, pt, patch, metav1.PatchOptions{})
	}
	if err != nil {
		return nil, apiErr(err)
	}
	return okResult(), nil
}

// resourceTarget resolves the (session, kind, name) a single-object action acts
// on, validating the required name param.
func resourceTarget(rc *plugin.RequestContext) (*Session, kind, string, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, kind{}, "", err
	}
	k, err := resolveKind(s, rc.Param("kind"))
	if err != nil {
		return nil, kind{}, "", err
	}
	name := rc.Param("name")
	if name == "" {
		return nil, kind{}, "", fmt.Errorf("%w: name is required", plugin.ErrInvalidInput)
	}
	if k.namespaced {
		if err := validateNamespace(rc.Param("namespace")); err != nil {
			return nil, kind{}, "", err
		}
	}
	return s, k, name, nil
}

func okResult() map[string]any { return map[string]any{"ok": true} }

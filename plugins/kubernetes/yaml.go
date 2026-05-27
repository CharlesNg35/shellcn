package kubernetes

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/charlesng/shellcn/internal/plugin"
)

// GetYAML returns a resource as editable YAML (managedFields/status stripped).
func GetYAML(rc *plugin.RequestContext) (any, error) {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return nil, err
	}
	o, err := s.get(rc, k, name)
	if err != nil {
		return nil, apiErr(err)
	}
	unstructured.RemoveNestedField(o.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(o.Object, "status")
	if k.redact {
		unstructured.RemoveNestedField(o.Object, "data")
	}
	out, err := yaml.Marshal(o.Object)
	if err != nil {
		return nil, err
	}
	return string(out), nil
}

// TemplateYAML builds a starter manifest for a kind, derived dynamically from the
// resolved GroupVersionKind (and the scoped namespace) — never a hardcoded blob.
func TemplateYAML(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	k, err := resolveKind(s, rc.Param("kind"))
	if err != nil {
		return nil, err
	}
	gvk, err := s.Mapper().KindFor(k.gvr)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve kind: %v", plugin.ErrUnavailable, err)
	}
	meta := map[string]any{"name": "example"}
	if k.namespaced {
		ns := rc.Param("namespace")
		if ns == "" {
			ns = s.defaultNS
		}
		if ns == "" {
			ns = "default"
		}
		meta["namespace"] = ns
	}
	body := obj{"spec": obj{}}
	if isCRD(k) {
		if sk, ok := crdSkeleton(rc, s, k.gvr); ok {
			body = sk
		}
	}
	body["apiVersion"] = gvk.GroupVersion().String()
	body["kind"] = gvk.Kind
	body["metadata"] = meta
	delete(body, "status")
	out, err := yaml.Marshal(body)
	if err != nil {
		return nil, err
	}
	return string(out), nil
}

// ApplyRequest is the code_editor save body, optionally a dry-run.
type ApplyRequest struct {
	Content string `json:"content"`
	DryRun  bool   `json:"dryRun"`
}

// ApplyYAML server-side-applies an arbitrary manifest (create or update). The
// GVK/namespace come from the document; the RESTMapper resolves the resource.
func ApplyYAML(rc *plugin.RequestContext) (any, error) {
	var req ApplyRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var content map[string]any
	if err := yaml.Unmarshal([]byte(req.Content), &content); err != nil {
		return nil, fmt.Errorf("%w: parse YAML: %v", plugin.ErrInvalidInput, err)
	}
	o := &unstructured.Unstructured{Object: content}
	gvk := o.GroupVersionKind()
	if gvk.Kind == "" || o.GetName() == "" {
		return nil, fmt.Errorf("%w: document needs apiVersion, kind, and metadata.name", plugin.ErrInvalidInput)
	}
	mapping, err := s.Mapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, apiErr(err)
	}
	ri := s.Dynamic().Resource(mapping.Resource)
	target := ri.Namespace(o.GetNamespace())
	if mapping.Scope.Name() != meta.RESTScopeNameNamespace {
		target = ri
	} else if o.GetNamespace() == "" {
		ns := s.defaultNS
		if ns == "" {
			ns = "default"
		}
		o.SetNamespace(ns)
		target = ri.Namespace(ns)
	}

	data, err := o.MarshalJSON()
	if err != nil {
		return nil, err
	}
	force := true
	opts := metav1.PatchOptions{FieldManager: "shellcn", Force: &force}
	if req.DryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	applied, err := target.Patch(rc.Ctx, o.GetName(), types.ApplyPatchType, data, opts)
	if err != nil {
		return nil, apiErr(err)
	}
	return map[string]any{
		"ok":        true,
		"kind":      gvk.Kind,
		"name":      applied.GetName(),
		"namespace": applied.GetNamespace(),
		"dryRun":    req.DryRun,
	}, nil
}

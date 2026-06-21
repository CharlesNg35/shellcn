package kubernetes

import (
	"errors"
	"fmt"
	"io"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// GetYAML returns a resource as editable YAML (server-managed fields stripped).
func GetYAML(rc *plugin.RequestContext) (any, error) {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return nil, err
	}
	o, err := s.get(rc, k, name)
	if err != nil {
		return nil, apiErr(err)
	}
	cleanForEdit(o)
	if k.redact {
		unstructured.RemoveNestedField(o.Object, "data")
		unstructured.RemoveNestedField(o.Object, "stringData")
	}
	return toYAML(o)
}

// cleanForEdit strips server-managed and cluster-assigned fields so the manifest
// round-trips cleanly through server-side apply. A resourceVersion left in the body
// acts as an optimistic-concurrency precondition, so a stale editor would fail every
// re-apply after the first; the other fields are noise the apiserver re-derives.
func cleanForEdit(o *unstructured.Unstructured) {
	for _, path := range [][]string{
		{"metadata", "resourceVersion"},
		{"metadata", "uid"},
		{"metadata", "creationTimestamp"},
		{"metadata", "generation"},
		{"metadata", "selfLink"},
		{"metadata", "managedFields"},
		{"status"},
	} {
		unstructured.RemoveNestedField(o.Object, path...)
	}
	annotations, _, _ := unstructured.NestedMap(o.Object, "metadata", "annotations")
	if annotations == nil {
		return
	}
	delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	if len(annotations) == 0 {
		unstructured.RemoveNestedField(o.Object, "metadata", "annotations")
		return
	}
	_ = unstructured.SetNestedMap(o.Object, annotations, "metadata", "annotations")
}

func toYAML(o *unstructured.Unstructured) (string, error) {
	out, err := yaml.Marshal(o.Object)
	if err != nil {
		return "", err
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

// ApplyYAML server-side-applies one or more manifests (a multi-document YAML
// stream, like `kubectl apply -f`). Each document's GVK/namespace come from the
// document; the RESTMapper resolves the resource. Documents apply in order; an
// error stops the run, leaving earlier documents applied.
func ApplyYAML(rc *plugin.RequestContext) (any, error) {
	var req ApplyRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	docs, err := decodeManifests(req.Content)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("%w: no manifests found", plugin.ErrInvalidInput)
	}

	results := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		res, err := s.applyManifest(rc, &unstructured.Unstructured{Object: doc}, req.DryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	if len(results) == 1 {
		return results[0], nil
	}
	// Join the canonical documents so the editor's RefreshField resets to the whole
	// applied manifest (and a multi-doc dry-run can still diff).
	parts := make([]string, 0, len(results))
	for _, r := range results {
		if c, ok := r["content"].(string); ok {
			parts = append(parts, c)
		}
	}
	return map[string]any{
		"ok": true, "dryRun": req.DryRun, "count": len(results),
		"applied": results,
		"content": strings.Join(parts, "---\n"),
	}, nil
}

// decodeManifests splits a YAML/JSON stream into its documents, skipping blanks.
func decodeManifests(content string) ([]map[string]any, error) {
	dec := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(content), 4096)
	var docs []map[string]any
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return docs, nil
			}
			return nil, fmt.Errorf("%w: parse YAML: %v", plugin.ErrInvalidInput, err)
		}
		if len(doc) > 0 {
			docs = append(docs, doc)
		}
	}
}

// applyManifest server-side-applies a single object, defaulting a namespaced
// object's namespace to the connection default.
func (s *Session) applyManifest(rc *plugin.RequestContext, o *unstructured.Unstructured, dryRun bool) (map[string]any, error) {
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

	applied, err := s.replaceOrCreate(rc, target, o, dryRun)
	if err != nil {
		return nil, apiErr(err)
	}
	cleanForEdit(applied)
	if gvk.Kind == "Secret" && gvk.Group == "" {
		unstructured.RemoveNestedField(applied.Object, "data")
		unstructured.RemoveNestedField(applied.Object, "stringData")
	}
	content, err := toYAML(applied)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":        true,
		"kind":      gvk.Kind,
		"name":      applied.GetName(),
		"namespace": applied.GetNamespace(),
		"dryRun":    dryRun,
		"content":   content,
	}, nil
}

// replaceOrCreate matches kubectl edit/replace semantics: Update an existing object
// carrying a freshly read resourceVersion (retry once on a conflict), or Create a new
// one. Full-object replace avoids server-side apply's associative-list merge, which
// can synthesize duplicates (e.g. a renamed Service port colliding on name).
func (s *Session) replaceOrCreate(rc *plugin.RequestContext, target dynamic.ResourceInterface, o *unstructured.Unstructured, dryRun bool) (*unstructured.Unstructured, error) {
	var dry []string
	if dryRun {
		dry = []string{metav1.DryRunAll}
	}
	live, err := target.Get(rc.Ctx, o.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return target.Create(rc.Ctx, o, metav1.CreateOptions{DryRun: dry})
	}
	if err != nil {
		return nil, err
	}
	o.SetResourceVersion(live.GetResourceVersion())
	applied, err := target.Update(rc.Ctx, o, metav1.UpdateOptions{DryRun: dry})
	if !apierrors.IsConflict(err) {
		return applied, err
	}
	live, err = target.Get(rc.Ctx, o.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	o.SetResourceVersion(live.GetResourceVersion())
	return target.Update(rc.Ctx, o, metav1.UpdateOptions{DryRun: dry})
}

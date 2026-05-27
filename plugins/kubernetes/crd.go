package kubernetes

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/charlesng/shellcn/internal/plugin"
)

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

// crdParamPrefix tags a {kind} param that encodes a CRD's GVR rather than a
// built-in catalog name: "crd:<group>/<version>/<resource>".
const crdParamPrefix = "crd:"

// crdKind resolves a "crd:g/v/r" param to a kind backed by the dynamic client.
// Columns are generic (name/namespace/age) since a CRD's shape is unknown until
// runtime; its scope comes from discovery.
func crdKind(s *Session, name string) (kind, error) {
	rest, ok := strings.CutPrefix(name, crdParamPrefix)
	if !ok {
		return kind{}, fmt.Errorf("%w: unknown resource kind %q", plugin.ErrNotFound, name)
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return kind{}, fmt.Errorf("%w: malformed custom resource %q", plugin.ErrInvalidInput, name)
	}
	gvr := schema.GroupVersionResource{Group: parts[0], Version: parts[1], Resource: parts[2]}
	return kind{
		name:       name,
		title:      parts[2],
		gvr:        gvr,
		namespaced: crdNamespaced(s, gvr),
		columns:    []plugin.Column{nameCol(), nsCol(), ageCol()},
	}, nil
}

func crdNamespaced(s *Session, gvr schema.GroupVersionResource) bool {
	list, err := s.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return true // assume namespaced; a wrong guess only mis-scopes the list query
	}
	for _, r := range list.APIResources {
		if r.Name == gvr.Resource {
			return r.Namespaced
		}
	}
	return true
}

// crdNode builds a tree node opening a CRD's (dynamic) instance list, or
// reports ok=false if the CRD has no servable resource/version.
func crdNode(o obj) (plugin.TreeNode, bool) {
	resource := str(o, "spec", "names", "plural")
	version := crdServedVersion(o)
	if resource == "" || version == "" {
		return plugin.TreeNode{}, false
	}
	param := fmt.Sprintf("%s%s/%s/%s", crdParamPrefix, str(o, "spec", "group"), version, resource)
	return plugin.TreeNode{
		Key:          "crd:" + str(o, "metadata", "name"),
		Label:        str(o, "spec", "names", "kind"),
		Icon:         lucide("puzzle"),
		Leaf:         true,
		ResourceKind: customResourceKind,
		ListParams:   map[string]string{"kind": param},
	}, true
}

// crdNodes lists CRDs as tree nodes, optionally filtered by API group.
func (s *Session) crdNodes(rc *plugin.RequestContext, groupMatch func(string) bool) ([]plugin.TreeNode, error) {
	list, err := s.Dynamic().Resource(crdGVR).List(rc.Ctx, metav1.ListOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	nodes := make([]plugin.TreeNode, 0, len(list.Items))
	for i := range list.Items {
		o := list.Items[i].Object
		if groupMatch != nil && !groupMatch(str(o, "spec", "group")) {
			continue
		}
		if n, ok := crdNode(o); ok {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// TreeCRDs lists installed CRDs (plus a "Definitions" entry) under Custom Resources.
func TreeCRDs(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	nodes, err := s.crdNodes(rc, nil)
	if err != nil {
		return nil, err
	}
	nodes = append([]plugin.TreeNode{{
		Key:          "crd-definitions",
		Label:        "Definitions",
		Icon:         lucide("list"),
		Leaf:         true,
		ResourceKind: "customresourcedefinition",
	}}, nodes...)
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
}

// TreeGatewayAPI nests the Gateway API CRD kinds under Network.
func TreeGatewayAPI(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	nodes, err := s.crdNodes(rc, func(group string) bool {
		return strings.Contains(group, "gateway.networking.k8s.io")
	})
	if err != nil {
		return nil, err
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
}

// crdSkeleton builds a starter object for a custom resource by sampling the
// CRD's OpenAPI v3 schema (required fields and declared defaults) of the served
// version — so Create is derived from the definition, not a fixed blob.
func crdSkeleton(rc *plugin.RequestContext, s *Session, gvr schema.GroupVersionResource) (obj, bool) {
	def, err := s.Dynamic().Resource(crdGVR).Get(rc.Ctx, gvr.Resource+"."+gvr.Group, metav1.GetOptions{})
	if err != nil {
		return nil, false
	}
	for _, v := range slice(def.Object, "spec", "versions") {
		vm, ok := v.(obj)
		if !ok || str(vm, "name") != gvr.Version {
			continue
		}
		if schemaObj := mapField(vm, "schema", "openAPIV3Schema"); schemaObj != nil {
			return sampleObject(schemaObj), true
		}
	}
	return nil, false
}

// sampleObject builds a minimal example from an OpenAPI v3 object schema: its
// required properties plus any carrying a declared default, recursively.
func sampleObject(schemaObj obj) obj {
	out := obj{}
	required := map[string]bool{}
	for _, r := range slice(schemaObj, "required") {
		if name, ok := r.(string); ok {
			required[name] = true
		}
	}
	for name, raw := range mapField(schemaObj, "properties") {
		ps, ok := raw.(obj)
		if !ok {
			continue
		}
		if _, hasDefault := ps["default"]; required[name] || hasDefault {
			out[name] = sampleValue(ps)
		}
	}
	return out
}

// sampleValue derives a placeholder for one schema node: its default if present,
// else a zero value matching the declared type (the first enum value for strings).
func sampleValue(schemaObj obj) any {
	if d, ok := schemaObj["default"]; ok {
		return d
	}
	switch str(schemaObj, "type") {
	case "array":
		return []any{}
	case "boolean":
		return false
	case "integer", "number":
		return 0
	case "string":
		if en := slice(schemaObj, "enum"); len(en) > 0 {
			return en[0]
		}
		return ""
	case "object":
		return sampleObject(schemaObj)
	default:
		if mapField(schemaObj, "properties") != nil {
			return sampleObject(schemaObj)
		}
		return obj{}
	}
}

// crdServedVersion prefers the storage version, else the first served one.
func crdServedVersion(o obj) string {
	var served string
	for _, v := range slice(o, "spec", "versions") {
		vm, ok := v.(obj)
		if !ok || !boolField(vm, "served") {
			continue
		}
		name := str(vm, "name")
		if boolField(vm, "storage") {
			return name
		}
		if served == "" {
			served = name
		}
	}
	return served
}

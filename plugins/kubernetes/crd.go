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

// TreeCRDs lists installed CustomResourceDefinitions as nodes that open each
// custom kind's (dynamic) list view.
func TreeCRDs(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	list, err := s.Dynamic().Resource(crdGVR).List(rc.Ctx, metav1.ListOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	nodes := make([]plugin.TreeNode, 0, len(list.Items)+1)
	// "Definitions" opens the list of CRDs themselves (Lens parity).
	nodes = append(nodes, plugin.TreeNode{
		Key:          "crd-definitions",
		Label:        "Definitions",
		Icon:         lucide("list"),
		Leaf:         true,
		ResourceKind: "customresourcedefinition",
	})
	for i := range list.Items {
		o := list.Items[i].Object
		group := str(o, "spec", "group")
		resource := str(o, "spec", "names", "plural")
		display := str(o, "spec", "names", "kind")
		version := crdServedVersion(o)
		if resource == "" || version == "" {
			continue
		}
		param := fmt.Sprintf("%s%s/%s/%s", crdParamPrefix, group, version, resource)
		nodes = append(nodes, plugin.TreeNode{
			Key:          "crd:" + str(o, "metadata", "name"),
			Label:        display,
			Icon:         plugin.Icon{Type: plugin.IconLucide, Value: "puzzle"},
			Leaf:         true,
			ResourceKind: customResourceKind,
			ListParams:   map[string]string{"kind": param},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
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

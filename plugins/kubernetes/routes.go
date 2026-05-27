package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng/shellcn/internal/plugin"
)

// Routes wires the Kubernetes route handlers. Step 1 establishes connectivity
// (list namespaces over both transports); later steps add the generic resource,
// stream, apply, and metrics routes.
func Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "kubernetes.namespaces.tree", Method: plugin.MethodGet, Path: "/tree/namespaces", Permission: "kubernetes.namespaces.read", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.namespaces.tree", Handle: TreeNamespaces},
		{ID: "kubernetes.namespaces.list", Method: plugin.MethodGet, Path: "/namespaces", Permission: "kubernetes.namespaces.read", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.namespaces.list", Handle: ListNamespaces},
		{ID: "kubernetes.namespace.get", Method: plugin.MethodGet, Path: "/namespaces/{name}", Permission: "kubernetes.namespaces.read", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.namespace.get", Handle: GetNamespace},
	}
}

func sess(rc *plugin.RequestContext) (*Session, error) { return Unwrap(rc.Session) }

// ListNamespaces returns the cluster's namespaces as grid rows. This is the
// Step 1 connectivity proof: it succeeds identically over direct (kubeconfig)
// and agent (in-cluster ServiceAccount) transport.
func ListNamespaces(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	list, err := s.Clientset().CoreV1().Namespaces().List(rc.Ctx, metav1.ListOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	return pageRows(rc, namespaceRows(list.Items))
}

// GetNamespace returns one namespace object for the detail document view.
func GetNamespace(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	ns, err := s.Clientset().CoreV1().Namespaces().Get(rc.Ctx, rc.Param("name"), metav1.GetOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	ns.ManagedFields = nil
	return ns, nil
}

// TreeNamespaces lists namespaces as selectable sidebar nodes.
func TreeNamespaces(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	list, err := s.Clientset().CoreV1().Namespaces().List(rc.Ctx, metav1.ListOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	nodes := make([]plugin.TreeNode, 0, len(list.Items))
	for i := range list.Items {
		ns := &list.Items[i]
		nodes = append(nodes, plugin.TreeNode{
			Key:   "namespace:" + ns.Name,
			Label: ns.Name,
			Icon:  plugin.Icon{Type: plugin.IconLucide, Value: "box"},
			Ref:   &plugin.ResourceRef{Kind: "namespace", Name: ns.Name, UID: ns.Name},
			Leaf:  true,
			Data: Row{
				"name":   ns.Name,
				"status": string(ns.Status.Phase),
			},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
}

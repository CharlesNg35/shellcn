package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// tree builds the Lens-style cluster menu: top-level Nodes, the expandable
// Workloads/Config/Network/Storage categories, top-level Namespaces and Events,
// then Access Control and Custom Resources.
func tree() []plugin.TreeGroup {
	groups := []plugin.TreeGroup{
		{
			Key:          "overview",
			Label:        "Overview",
			Icon:         lucide("layout-dashboard"),
			Source:       plugin.DataSource{RouteID: "kubernetes.cluster.tree"},
			ResourceKind: clusterKind,
		},
		kindGroup("node", "Nodes", "server"),
	}
	for _, key := range []string{"workloads", "config", "network", "storage"} {
		groups = append(groups, categoryGroup(key))
	}
	groups = append(groups,
		kindGroup("namespace", "Namespaces", "box"),
		kindGroup("event", "Events", "bell"),
		plugin.TreeGroup{
			Key:    "helm",
			Label:  "Helm",
			Icon:   plugin.Icon{Type: plugin.IconLucide, Value: "ship-wheel"},
			Source: plugin.DataSource{RouteID: "kubernetes.tree.helm"},
		},
		categoryGroup("access"),
		plugin.TreeGroup{
			Key:    "customresources",
			Label:  "Custom Resources",
			Icon:   plugin.Icon{Type: plugin.IconLucide, Value: "puzzle"},
			Source: plugin.DataSource{RouteID: "kubernetes.tree.crds"},
		},
	)
	return groups
}

// kindGroup is a top-level group that expands to a kind's instances and opens
// that kind's list on click.
func kindGroup(kindName, label, icon string) plugin.TreeGroup {
	return plugin.TreeGroup{
		Key:          kindName + "s",
		Label:        label,
		Icon:         plugin.Icon{Type: plugin.IconLucide, Value: icon},
		Source:       plugin.DataSource{RouteID: "kubernetes.tree.kind", Params: map[string]string{"kind": kindName}},
		ResourceKind: kindName,
	}
}

func categoryGroup(key string) plugin.TreeGroup {
	for _, c := range categories {
		if c.key == key {
			return plugin.TreeGroup{
				Key:    c.key,
				Label:  c.label,
				Icon:   plugin.Icon{Type: plugin.IconLucide, Value: c.icon},
				Source: plugin.DataSource{RouteID: "kubernetes.tree.category", Params: map[string]string{"category": c.key}},
			}
		}
	}
	return plugin.TreeGroup{}
}

func kindLeaf(k kind) plugin.TreeNode {
	return plugin.TreeNode{
		Key:          "kind:" + k.name,
		Label:        k.title,
		Icon:         plugin.Icon{Type: plugin.IconLucide, Value: k.icon},
		Leaf:         true,
		ResourceKind: k.name,
	}
}

// TreeCategory returns a category's kinds as leaves plus any nested sub-groups
// (e.g. Admission Policies under Config, Gateway API under Network).
func TreeCategory(rc *plugin.RequestContext) (any, error) {
	cat := rc.Param("category")
	nodes := make([]plugin.TreeNode, 0)
	seenSub := map[string]bool{}
	for _, k := range kinds {
		if k.category != cat {
			continue
		}
		if k.subgroup != "" {
			if !seenSub[k.subgroup] {
				seenSub[k.subgroup] = true
				nodes = append(nodes, plugin.TreeNode{
					Key:            cat + ":" + k.subgroup,
					Label:          subgroupLabels[k.subgroup],
					Icon:           plugin.Icon{Type: plugin.IconLucide, Value: "folder"},
					ChildrenSource: &plugin.DataSource{RouteID: "kubernetes.tree.subgroup", Params: map[string]string{"subgroup": k.subgroup}},
				})
			}
			continue
		}
		nodes = append(nodes, kindLeaf(k))
	}
	if cat == "network" {
		nodes = append(nodes, plugin.TreeNode{
			Key:            "network:gatewayapi",
			Label:          "Gateway API",
			Icon:           plugin.Icon{Type: plugin.IconLucide, Value: "globe"},
			ChildrenSource: &plugin.DataSource{RouteID: "kubernetes.tree.gatewayapi"},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
}

// TreeSubgroup returns the kinds belonging to a nested sub-group.
func TreeSubgroup(rc *plugin.RequestContext) (any, error) {
	sub := rc.Param("subgroup")
	nodes := make([]plugin.TreeNode, 0)
	for _, k := range kinds {
		if k.subgroup == sub {
			nodes = append(nodes, kindLeaf(k))
		}
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
}

// treeInstanceLimit caps how many instances a top-level kind group expands to.
const treeInstanceLimit = 200

// TreeKindInstances returns a kind's instances as selectable detail nodes.
func TreeKindInstances(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	k, err := resolveKind(s, rc.Param("kind"))
	if err != nil {
		return nil, err
	}
	ri := s.Dynamic().Resource(k.gvr)
	opts := metav1.ListOptions{Limit: treeInstanceLimit}
	ul, err := ri.List(rc.Ctx, opts)
	if ns := s.listNamespace(rc, k); k.namespaced && ns != "" {
		ul, err = ri.Namespace(ns).List(rc.Ctx, opts)
	}
	if err != nil {
		return nil, apiErr(err)
	}
	nodes := make([]plugin.TreeNode, 0, len(ul.Items))
	for i := range ul.Items {
		o := ul.Items[i].Object
		row := commonRow(o)
		if k.extra != nil {
			for key, val := range k.extra(o) {
				row[key] = val
			}
		}
		nodes = append(nodes, plugin.TreeNode{
			Key:   k.name + ":" + str(o, "metadata", "uid"),
			Label: refName(o),
			Icon:  plugin.Icon{Type: plugin.IconLucide, Value: k.icon},
			Ref:   &plugin.ResourceRef{Kind: k.name, Namespace: refNS(o), Name: refName(o), UID: str(o, "metadata", "uid")},
			Leaf:  true,
			Data:  row,
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
}

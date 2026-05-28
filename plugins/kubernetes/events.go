package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/charlesng35/shellcn/internal/plugin"
)

var eventGVR = schema.GroupVersionResource{Version: "v1", Resource: "events"}

// ResourceEvents lists the events involving one object, for its detail Events tab.
func ResourceEvents(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	name := rc.Param("name")
	if name == "" {
		return pageRows(rc, nil)
	}
	ri := s.Dynamic().Resource(eventGVR)
	opts := metav1.ListOptions{FieldSelector: "involvedObject.name=" + name, Limit: 500}
	var (
		list *unstructured.UnstructuredList
		err2 error
	)
	if ns := rc.Param("namespace"); ns != "" {
		list, err2 = ri.Namespace(ns).List(rc.Ctx, opts)
	} else {
		list, err2 = ri.List(rc.Ctx, opts)
	}
	if err2 != nil {
		return nil, apiErr(err2)
	}
	rows := make([]Row, 0, len(list.Items))
	for i := range list.Items {
		o := list.Items[i].Object
		row := commonRow(o)
		for key, val := range eventRow(o) {
			row[key] = val
		}
		row["ref"] = plugin.ResourceRef{Kind: "event", Namespace: refNS(o), Name: refName(o), UID: str(o, "metadata", "uid")}
		rows = append(rows, row)
	}
	sortRecentFirst(rows)
	return pageRows(rc, rows)
}

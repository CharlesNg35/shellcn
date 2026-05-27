package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/charlesng/shellcn/internal/plugin"
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
	list, err := s.Dynamic().Resource(eventGVR).List(rc.Ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + name,
	})
	if err != nil {
		return nil, apiErr(err)
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
	return pageRows(rc, rows)
}

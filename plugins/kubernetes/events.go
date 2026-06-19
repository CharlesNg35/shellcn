package kubernetes

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/charlesng35/shellcn/sdk/plugin"
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
		rows = append(rows, eventToRow(list.Items[i].Object))
	}
	sortRecentFirst(rows)
	return pageRows(rc, rows)
}

func eventToRow(o obj) Row {
	row := commonRow(o)
	for key, val := range eventRow(o) {
		row[key] = val
	}
	row["ref"] = plugin.ResourceIdentity{Kind: "event", Namespace: refNS(o), Name: refName(o), UID: str(o, "metadata", "uid")}
	return row
}

// WatchEvents streams live events involving one object so its Events timeline
// updates in place instead of polling.
func WatchEvents(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	name := rc.Param("name")
	if name == "" {
		return nil
	}
	hub := s.liveHub()
	if hub == nil {
		return nil
	}
	key := feedKey{GVR: eventGVR, FieldSelector: "involvedObject.name=" + name}
	if ns := rc.Param("namespace"); ns != "" {
		key.Namespace = ns
	}
	events, unsubscribe := hub.Subscribe(key)
	defer unsubscribe()

	enc := json.NewEncoder(client)
	for {
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			u, ok := ev.Object.(interface{ UnstructuredContent() map[string]any })
			if !ok {
				continue
			}
			o := u.UnstructuredContent()
			frame := &plugin.ResourceEvent{
				Type:     resourceEventType(ev.Type),
				Ref:      plugin.ResourceIdentity{Kind: "event", Namespace: refNS(o), Name: refName(o), UID: str(o, "metadata", "uid")},
				Resource: eventToRow(o),
			}
			if err := enc.Encode(frame); err != nil {
				return nil
			}
		}
	}
}

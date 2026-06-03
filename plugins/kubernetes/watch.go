package kubernetes

import (
	"encoding/json"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// WatchResource streams live ResourceEvents for a kind so the generic table/tree
// patches in place. It uses the API watch directly (the dynamic client's Watch),
// scoped to the same namespace as the list.
func WatchResource(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	k, err := resolveKind(s, rc.Param("kind"))
	if err != nil {
		return err
	}

	ri := s.Dynamic().Resource(k.gvr)
	opts := metav1.ListOptions{}
	var w watch.Interface
	if ns := s.listNamespace(rc, k); k.namespaced && ns != "" {
		w, err = ri.Namespace(ns).Watch(rc.Ctx, opts)
	} else {
		w, err = ri.Watch(rc.Ctx, opts)
	}
	if err != nil {
		return apiErr(err)
	}
	defer w.Stop()

	enc := json.NewEncoder(client)
	for {
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case ev, ok := <-w.ResultChan():
			if !ok {
				return nil
			}
			frame := watchFrame(k, ev)
			if frame == nil {
				continue
			}
			if err := enc.Encode(frame); err != nil {
				return nil
			}
		}
	}
}

func watchFrame(k kind, ev watch.Event) *plugin.ResourceEvent {
	u, ok := ev.Object.(interface{ UnstructuredContent() map[string]any })
	if !ok {
		return nil
	}
	o := u.UnstructuredContent()
	row := commonRow(o)
	if k.extra != nil {
		for key, val := range k.extra(o) {
			row[key] = val
		}
	}
	return &plugin.ResourceEvent{
		// k8s emits ADDED/MODIFIED/DELETED; the renderer's contract is lowercase.
		Type: strings.ToLower(string(ev.Type)),
		Ref: plugin.ResourceRef{
			Kind:      k.name,
			Namespace: refNS(o),
			Name:      refName(o),
			UID:       str(o, "metadata", "uid"),
		},
		Resource: row,
	}
}

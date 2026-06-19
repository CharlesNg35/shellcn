package kubernetes

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// WatchResource streams a kind's live ResourceEvents so the generic table/tree
// patches in place, scoped to the same namespace as the list.
func WatchResource(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	k, err := resolveKind(s, rc.Param("kind"))
	if err != nil {
		return err
	}

	hub := s.liveHub()
	if hub == nil {
		return nil // session closing
	}
	key := feedKey{GVR: k.gvr}
	if ns := s.listNamespace(rc, k); k.namespaced && ns != "" {
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

// WatchObject streams one resource as the same record shape the overview GET
// returns, so the detail panel live-updates in place.
func WatchObject(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return err
	}
	// RBAC is computed once per connection (it doesn't change mid-session) and
	// merged into every frame so action gating stays consistent across live updates.
	can := s.accessReview(rc.Ctx, k, rc.Param("namespace"), name)
	return streamObject(rc, client, s, k, name, func(o obj) any {
		out := overviewRecord(k, o)
		if k.redact {
			out["redacted"] = true
			delete(out, "data")
			delete(out, "stringData")
		}
		for key, allowed := range can {
			out[key] = allowed
		}
		return out
	})
}

// WatchObjectYAML streams one resource as clean editable YAML, matching GetYAML so
// the editor's live content equals a fresh load.
func WatchObjectYAML(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return err
	}
	return streamObject(rc, client, s, k, name, func(o obj) any {
		u := &unstructured.Unstructured{Object: o}
		cleanForEdit(u)
		if k.redact {
			unstructured.RemoveNestedField(u.Object, "data")
		}
		content, err := toYAML(u)
		if err != nil {
			return ""
		}
		return content
	})
}

// streamObject subscribes to the single-object hub feed and encodes each change as
// a ResourceEvent whose payload is produced by render.
func streamObject(rc *plugin.RequestContext, client plugin.ClientStream, s *Session, k kind, name string, render func(obj) any) error {
	hub := s.liveHub()
	if hub == nil {
		return nil
	}
	key := feedKey{GVR: k.gvr, FieldSelector: "metadata.name=" + name}
	if ns := rc.Param("namespace"); k.namespaced && ns != "" {
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
				Type: resourceEventType(ev.Type),
				Ref: plugin.ResourceIdentity{
					Kind:      k.name,
					Namespace: refNS(o),
					Name:      refName(o),
					UID:       str(o, "metadata", "uid"),
				},
				Resource: render(o),
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
		Type: resourceEventType(ev.Type),
		Ref: plugin.ResourceIdentity{
			Kind:      k.name,
			Namespace: refNS(o),
			Name:      refName(o),
			UID:       str(o, "metadata", "uid"),
		},
		Resource: row,
	}
}

func resourceEventType(t watch.EventType) string {
	switch t {
	case watch.Added:
		return "added"
	case watch.Deleted:
		return "deleted"
	default:
		return "updated"
	}
}

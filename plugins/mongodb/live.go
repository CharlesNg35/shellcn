package mongodb

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const mongoResourceWatchInterval = 5 * time.Second

type mongoWatchedRow struct {
	ref         plugin.ResourceIdentity
	row         plugin.TableRow
	fingerprint string
}

func watchResource(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := mongoSession(rc)
	if err != nil {
		return err
	}
	kind := strings.TrimSpace(rc.Param("kind"))
	if kind == "" {
		kind = strings.TrimSpace(rc.Query().Get("kind"))
	}
	if kind == "" {
		return fmt.Errorf("%w: kind is required", plugin.ErrInvalidInput)
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(mongoResourceWatchInterval)
	defer ticker.Stop()

	prev, err := mongoWatchedRows(rc, s, kind)
	if err != nil {
		return err
	}
	for {
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
			next, err := mongoWatchedRows(rc, s, kind)
			if err != nil {
				return err
			}
			events := diffMongoWatchedRows(prev, next)
			prev = next
			for _, event := range events {
				if err := enc.Encode(event); err != nil {
					return nil
				}
			}
		}
	}
}

func mongoWatchedRows(rc *plugin.RequestContext, s *Session, kind string) (map[string]mongoWatchedRow, error) {
	var rows []plugin.TableRow
	var err error
	switch kind {
	case "server":
		doc, err := serverStatusDoc(rc, s)
		if err != nil {
			return nil, err
		}
		row := serverStatusRow(doc)
		row["ref"] = plugin.ResourceIdentity{Kind: "server", Name: "MongoDB", UID: "server"}
		rows = []plugin.TableRow{row}
	case "database":
		rows, err = databaseRows(rc.Ctx, s)
	case "collection":
		rows, err = collectionRows(rc.Ctx, s, queryParam(rc, "database"))
	case "index":
		var database string
		database, err = safeName(queryParam(rc, "database"), "database")
		if err != nil {
			return nil, err
		}
		var collection string
		collection, err = safeName(queryParam(rc, "collection"), "collection")
		if err != nil {
			return nil, err
		}
		rows, err = indexRows(rc.Ctx, s, database, collection)
	default:
		return nil, fmt.Errorf("%w: unsupported watch kind %q", plugin.ErrInvalidInput, kind)
	}
	if err != nil {
		return nil, err
	}
	return indexMongoWatchedRows(rows), nil
}

func queryParam(rc *plugin.RequestContext, key string) string {
	if v := strings.TrimSpace(rc.Query().Get("p." + key)); v != "" {
		return v
	}
	return strings.TrimSpace(rc.Query().Get(key))
}

func indexMongoWatchedRows(rows []plugin.TableRow) map[string]mongoWatchedRow {
	out := make(map[string]mongoWatchedRow, len(rows))
	for _, row := range rows {
		ref, ok := row["ref"].(plugin.ResourceIdentity)
		if !ok || ref.Kind == "" || ref.UID == "" {
			continue
		}
		key := ref.Kind + ":" + ref.Namespace + ":" + ref.Scope + ":" + ref.UID
		out[key] = mongoWatchedRow{ref: ref, row: row, fingerprint: mongoFingerprint(row)}
	}
	return out
}

func diffMongoWatchedRows(prev, next map[string]mongoWatchedRow) []plugin.ResourceEvent {
	events := make([]plugin.ResourceEvent, 0)
	for key, row := range next {
		old, ok := prev[key]
		if !ok {
			events = append(events, plugin.ResourceEvent{Type: "added", Ref: row.ref, Resource: row.row})
			continue
		}
		if old.fingerprint != row.fingerprint {
			events = append(events, plugin.ResourceEvent{Type: "updated", Ref: row.ref, Resource: row.row})
		}
	}
	for key, row := range prev {
		if _, ok := next[key]; !ok {
			events = append(events, plugin.ResourceEvent{Type: "deleted", Ref: row.ref})
		}
	}
	return events
}

func mongoFingerprint(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}

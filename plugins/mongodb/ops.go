package mongodb

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const mongoMetricsInterval = 3 * time.Second

func serverList(rc *plugin.RequestContext) (any, error) {
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	doc, err := serverStatusDoc(rc, s)
	if err != nil {
		return nil, err
	}
	row := serverStatusRow(doc)
	row["ref"] = plugin.ResourceIdentity{Kind: "server", Name: "MongoDB", UID: "server"}
	return plugin.Page[plugin.TableRow]{Items: []plugin.TableRow{row}, Total: intPtr(1)}, nil
}

func serverStatus(rc *plugin.RequestContext) (any, error) {
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	doc, err := serverStatusDoc(rc, s)
	if err != nil {
		return nil, err
	}
	return serverStatusRow(doc), nil
}

func serverMetrics(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := mongoSession(rc)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(mongoMetricsInterval)
	defer ticker.Stop()

	var prev plugin.TableRow
	var prevAt time.Time
	for {
		doc, err := serverStatusDoc(rc, s)
		if err != nil {
			return err
		}
		frame := serverMetricsFrame(doc)
		now := time.Now()
		if prev != nil {
			dt := now.Sub(prevAt).Seconds()
			addMongoRate(frame, prev, "networkBytesIn", "networkInRate", dt)
			addMongoRate(frame, prev, "networkBytesOut", "networkOutRate", dt)
			for _, key := range []string{"insert", "query", "update", "delete", "getmore", "command"} {
				addMongoRate(frame, prev, "ops_"+key, "ops_"+key+"Rate", dt)
			}
		}
		prev, prevAt = frame, now
		if err := enc.Encode(frame); err != nil {
			return nil
		}
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func healthList(rc *plugin.RequestContext) (any, error) {
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	status, err := serverStatusDoc(rc, s)
	if err != nil {
		return nil, err
	}
	rows := healthRowsFromServer(status)
	collections, err := collectionRows(rc.Ctx, s, "")
	if err != nil {
		return nil, err
	}
	for _, collection := range collections {
		database := fmt.Sprint(collection["database"])
		name := fmt.Sprint(collection["name"])
		count := numberValue(collection["count"])
		indexes, err := indexRows(rc.Ctx, s, database, name)
		if err != nil {
			return nil, err
		}
		if count > 0 && nonPrimaryIndexCount(indexes) == 0 {
			rows = append(rows, healthRow("warn", database+"."+name, "Collection has documents but only the primary _id index.", "Add query-specific indexes for common filters and sort keys.", plugin.ResourceIdentity{Kind: "collection", Namespace: database, Name: name, UID: database + "." + name}))
		}
	}
	return pageRows(rc, rows)
}

func listCurrentOps(rc *plugin.RequestContext) (any, error) {
	s, err := mongoSession(rc)
	if err != nil {
		return nil, err
	}
	rows, err := currentOpRows(rc, s)
	if err != nil {
		return nil, err
	}
	return pageRows(rc, rows)
}

func serverStatusDoc(rc *plugin.RequestContext, s *Session) (bson.M, error) {
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	var doc bson.M
	if err := s.client.Database("admin").RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&doc); err != nil {
		return nil, mongoErr(err)
	}
	return doc, nil
}

func serverStatusRow(doc bson.M) plugin.TableRow {
	row := serverMetricsFrame(doc)
	row["status"] = "healthy"
	row["host"] = strValue(doc["host"])
	row["version"] = strValue(doc["version"])
	row["process"] = strValue(doc["process"])
	row["uptimeSeconds"] = numberValue(doc["uptime"])
	if repl := mapValue(doc["repl"]); repl != nil {
		row["replicaSet"] = strValue(repl["setName"])
		row["replicaState"] = strValue(repl["stateStr"])
	}
	return row
}

func serverMetricsFrame(doc bson.M) plugin.TableRow {
	row := plugin.TableRow{
		"uptimeSeconds": numberValue(doc["uptime"]),
	}
	if connections := mapValue(doc["connections"]); connections != nil {
		current := numberValue(connections["current"])
		available := numberValue(connections["available"])
		row["connectionsCurrent"] = current
		row["connectionsAvailable"] = available
		if total := current + available; total > 0 {
			row["connectionsPct"] = float64(current) / float64(total) * 100
		}
	}
	if mem := mapValue(doc["mem"]); mem != nil {
		row["memoryResidentMB"] = numberValue(mem["resident"])
		row["memoryVirtualMB"] = numberValue(mem["virtual"])
	}
	if network := mapValue(doc["network"]); network != nil {
		row["networkBytesIn"] = numberValue(network["bytesIn"])
		row["networkBytesOut"] = numberValue(network["bytesOut"])
		row["networkRequests"] = numberValue(network["numRequests"])
	}
	if ops := mapValue(doc["opcounters"]); ops != nil {
		for _, key := range []string{"insert", "query", "update", "delete", "getmore", "command"} {
			row["ops_"+key] = numberValue(ops[key])
		}
	}
	return row
}

func healthRowsFromServer(doc bson.M) []plugin.TableRow {
	rows := []plugin.TableRow{}
	if connections := mapValue(doc["connections"]); connections != nil {
		current := numberValue(connections["current"])
		available := numberValue(connections["available"])
		if total := current + available; total > 0 {
			pct := float64(current) / float64(total) * 100
			switch {
			case pct >= 90:
				rows = append(rows, healthRow("critical", "Connections", fmt.Sprintf("Connection utilization is %.0f%%.", pct), "Increase max connections or reduce client pooling pressure.", plugin.ResourceIdentity{}))
			case pct >= 80:
				rows = append(rows, healthRow("warn", "Connections", fmt.Sprintf("Connection utilization is %.0f%%.", pct), "Review client pooling and slow operations.", plugin.ResourceIdentity{}))
			}
		}
	}
	if globalLock := mapValue(doc["globalLock"]); globalLock != nil {
		if queue := mapValue(globalLock["currentQueue"]); queue != nil {
			total := numberValue(queue["total"])
			if total > 0 {
				rows = append(rows, healthRow("warn", "Global lock", fmt.Sprintf("%d operations are queued for locks.", total), "Inspect current operations and slow queries.", plugin.ResourceIdentity{}))
			}
		}
	}
	if asserts := mapValue(doc["asserts"]); asserts != nil {
		if msg := numberValue(asserts["msg"]); msg > 0 {
			rows = append(rows, healthRow("warn", "Assertions", fmt.Sprintf("%d server assertion messages are reported.", msg), "Check MongoDB logs for assertion details.", plugin.ResourceIdentity{}))
		}
	}
	return rows
}

func currentOpRows(rc *plugin.RequestContext, s *Session) ([]plugin.TableRow, error) {
	database := queryParam(rc, "database")
	collection := queryParam(rc, "collection")
	namespacePrefix := database
	if database != "" && collection != "" {
		namespacePrefix = database + "." + collection
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	var doc bson.M
	if err := s.client.Database("admin").RunCommand(ctx, bson.D{{Key: "currentOp", Value: 1}, {Key: "$all", Value: true}}).Decode(&doc); err != nil {
		return nil, mongoErr(err)
	}
	ops := arrayValue(doc["inprog"])
	rows := make([]plugin.TableRow, 0, len(ops))
	for _, value := range ops {
		op := anyMap(value)
		if op == nil {
			continue
		}
		opid := fmt.Sprint(op["opid"])
		if opid == "" || opid == "<nil>" {
			opid = fmt.Sprintf("op-%d", len(rows)+1)
		}
		row := plugin.TableRow{
			"opid":           opid,
			"active":         boolField(op["active"]),
			"secsRunning":    numberValue(op["secs_running"]),
			"op":             strValue(op["op"]),
			"namespace":      strValue(op["ns"]),
			"description":    strValue(op["desc"]),
			"client":         strValue(op["client"]),
			"appName":        strValue(op["appName"]),
			"waitingForLock": boolField(op["waitingForLock"]),
			"command":        compactJSON(op["command"]),
			"ref":            plugin.ResourceIdentity{Kind: "operation", Name: opid, UID: opid},
		}
		if row["namespace"] == "" {
			row["namespace"] = strValue(op["namespace"])
		}
		if namespacePrefix != "" && !strings.HasPrefix(fmt.Sprint(row["namespace"]), namespacePrefix) {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func healthRow(status, target, message, recommendation string, ref plugin.ResourceIdentity) plugin.TableRow {
	row := plugin.TableRow{
		"status":         status,
		"target":         target,
		"message":        message,
		"recommendation": recommendation,
	}
	if ref.Kind != "" {
		row["ref"] = ref
	}
	return row
}

func nonPrimaryIndexCount(rows []plugin.TableRow) int {
	count := 0
	for _, row := range rows {
		if fmt.Sprint(row["name"]) != "_id_" {
			count++
		}
	}
	return count
}

func addMongoRate(frame, prev plugin.TableRow, cumKey, rateKey string, dt float64) {
	if dt <= 0 {
		return
	}
	cur, ok1 := floatValue(frame[cumKey])
	old, ok2 := floatValue(prev[cumKey])
	if ok1 && ok2 && cur >= old {
		frame[rateKey] = (cur - old) / dt
	}
}

func mapValue(value any) map[string]any {
	return anyMap(value)
}

func anyMap(value any) map[string]any {
	switch v := value.(type) {
	case bson.M:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = item
		}
		return out
	case map[string]any:
		return v
	default:
		return nil
	}
}

func arrayValue(value any) []any {
	switch v := value.(type) {
	case bson.A:
		return []any(v)
	case []any:
		return v
	default:
		return nil
	}
}

func strValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func floatValue(value any) (float64, bool) {
	switch n := value.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func intPtr(v int) *int {
	return &v
}

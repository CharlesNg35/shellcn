package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
)

type keyEntry struct {
	Key  string `json:"key"`
	Type string `json:"type,omitempty"`
	TTL  int64  `json:"ttl,omitempty"`
	Size int64  `json:"size,omitempty"`
}

type keyDetail struct {
	Key      string `json:"key"`
	Type     string `json:"type,omitempty"`
	TTL      int64  `json:"ttl,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	Value    any    `json:"value,omitempty"`
}

type actionResult struct {
	OK bool `json:"ok"`
}

type confirmationError struct {
	message string
}

func (e confirmationError) Error() string { return e.message }

func routes() []plugin.Route {
	return []plugin.Route{
		{ID: "redis.overview", Method: plugin.MethodGet, Path: "/overview", Permission: "redis.read", Risk: plugin.RiskSafe, AuditEvent: "redis.overview", Handle: overview},
		{ID: "redis.info", Method: plugin.MethodGet, Path: "/info", Permission: "redis.read", Risk: plugin.RiskSafe, AuditEvent: "redis.info", Handle: infoRoute},
		{ID: "redis.keys.list", Method: plugin.MethodGet, Path: "/keys", Permission: "redis.keys.read", Risk: plugin.RiskSafe, AuditEvent: "redis.keys.list", Handle: listKeys},
		{ID: "redis.key.read", Method: plugin.MethodGet, Path: "/keys/{key}", Permission: "redis.keys.read", Risk: plugin.RiskSafe, AuditEvent: "redis.key.read", Handle: readKey},
		{ID: "redis.key.write", Method: plugin.MethodPut, Path: "/keys/{key}", Permission: "redis.keys.write", Risk: plugin.RiskWrite, AuditEvent: "redis.key.write", Handle: writeKey},
		{ID: "redis.key.delete", Method: plugin.MethodDelete, Path: "/keys/{key}", Permission: "redis.keys.delete", Risk: plugin.RiskDestructive, AuditEvent: "redis.key.delete", Handle: deleteKey},
		{ID: "redis.clients.list", Method: plugin.MethodGet, Path: "/clients", Permission: "redis.clients.read", Risk: plugin.RiskSafe, AuditEvent: "redis.clients.list", Handle: listClients},
		{ID: "redis.channels.list", Method: plugin.MethodGet, Path: "/channels", Permission: "redis.pubsub.read", Risk: plugin.RiskSafe, AuditEvent: "redis.channels.list", Handle: listChannels},
		{ID: "redis.terminal", Method: plugin.MethodWS, Path: "/terminal", Permission: "redis.command.execute", Risk: plugin.RiskPrivileged, AuditEvent: "redis.terminal", Stream: terminalStream},
		{ID: "redis.command", Method: plugin.MethodWS, Path: "/command", Permission: "redis.command.execute", Risk: plugin.RiskPrivileged, AuditEvent: "redis.command", Stream: commandStream},
		{ID: "redis.completion", Method: plugin.MethodGet, Path: "/completion", Permission: "redis.read", Risk: plugin.RiskSafe, AuditEvent: "redis.completion", Handle: completionRoute},
	}
}

func redisSession(rc *plugin.RequestContext) (*Session, error) {
	s, err := unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	return s, nil
}

func overview(rc *plugin.RequestContext) (any, error) {
	s, err := redisSession(rc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	info, err := s.client.Info(ctx, "server", "clients", "memory", "stats", "keyspace").Result()
	if err != nil {
		return nil, redisErr(err)
	}
	sections := parseInfo(info)
	db := "db" + strconv.Itoa(s.opts.Database)
	out := map[string]any{
		"database": s.opts.Database,
		"address":  s.client.Options().Addr,
		"readOnly": s.opts.ReadOnly,
	}
	for _, key := range []string{"redis_version", "redis_mode", "connected_clients", "used_memory_human", "total_commands_processed"} {
		if v, ok := sections[key]; ok {
			out[key] = v
		}
	}
	if v, ok := sections[db]; ok {
		out["keyspace"] = v
	}
	return out, nil
}

func infoRoute(rc *plugin.RequestContext) (any, error) {
	s, err := redisSession(rc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	info, err := s.client.Info(ctx).Result()
	if err != nil {
		return nil, redisErr(err)
	}
	return parseInfo(info), nil
}

func listKeys(rc *plugin.RequestContext) (any, error) {
	s, err := redisSession(rc)
	if err != nil {
		return nil, err
	}
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	pattern := strings.TrimSpace(req.Filter["q"])
	if pattern == "" {
		pattern = s.opts.KeyPattern
	}
	if !strings.ContainsAny(pattern, "*?[") {
		pattern = "*" + pattern + "*"
	}
	cursor, err := scanCursor(req.Cursor)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit > s.opts.ScanCount {
		limit = s.opts.ScanCount
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	keys, next, err := s.client.Scan(ctx, cursor, pattern, int64(limit)).Result()
	if err != nil {
		return nil, redisErr(err)
	}
	items := make([]keyEntry, 0, len(keys))
	for _, key := range keys {
		entry, err := keySummary(ctx, s, key)
		if err != nil {
			return nil, err
		}
		items = append(items, entry)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	nextCursor := ""
	if next != 0 {
		nextCursor = strconv.FormatUint(next, 10)
	}
	return plugin.Page[keyEntry]{Items: items, NextCursor: nextCursor}, nil
}

func readKey(rc *plugin.RequestContext) (any, error) {
	s, err := redisSession(rc)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(rc.Param("key"))
	if key == "" {
		return nil, fmt.Errorf("%w: key is required", plugin.ErrInvalidInput)
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	detail, err := keyValue(ctx, s, key)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func writeKey(rc *plugin.RequestContext) (any, error) {
	s, err := redisSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(rc.Param("key"))
	if key == "" {
		return nil, fmt.Errorf("%w: key is required", plugin.ErrInvalidInput)
	}
	var req struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	kind := normalizeType(req.Type)
	if kind == "" {
		kind = "string"
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	ttl, _ := s.client.TTL(ctx, key).Result()
	if err := replaceValue(ctx, s, key, kind, req.Value); err != nil {
		return nil, err
	}
	if ttl > 0 {
		if err := s.client.Expire(ctx, key, ttl).Err(); err != nil {
			return nil, redisErr(err)
		}
	}
	return actionResult{OK: true}, nil
}

func deleteKey(rc *plugin.RequestContext) (any, error) {
	s, err := redisSession(rc)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(s); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(rc.Param("key"))
	if key == "" {
		return nil, fmt.Errorf("%w: key is required", plugin.ErrInvalidInput)
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return nil, redisErr(err)
	}
	return actionResult{OK: true}, nil
}

func listClients(rc *plugin.RequestContext) (any, error) {
	s, err := redisSession(rc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	raw, err := s.client.ClientList(ctx).Result()
	if err != nil {
		return nil, redisErr(err)
	}
	rows := parseClientList(raw)
	return pageRows(rc, rows)
}

func listChannels(rc *plugin.RequestContext) (any, error) {
	s, err := redisSession(rc)
	if err != nil {
		return nil, err
	}
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	pattern := strings.TrimSpace(req.Filter["q"])
	if pattern == "" {
		pattern = "*"
	}
	ctx, cancel := commandContext(rc.Ctx, s)
	defer cancel()
	channels, err := s.client.PubSubChannels(ctx, pattern).Result()
	if err != nil {
		return nil, redisErr(err)
	}
	counts, err := s.client.PubSubNumSub(ctx, channels...).Result()
	if err != nil {
		return nil, redisErr(err)
	}
	rows := make([]map[string]any, 0, len(channels))
	for _, channel := range channels {
		rows = append(rows, map[string]any{"name": channel, "subscribers": counts[channel]})
	}
	return pageRows(rc, rows)
}

func terminalStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := redisSession(rc)
	if err != nil {
		return err
	}
	prompt := redisPrompt(s)
	if err := writeTerminal(client, "\r\nRedis console\r\n"+prompt); err != nil {
		return err
	}
	var line strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := client.Read(buf)
		if n > 0 {
			if len(buf[:n]) > 0 && buf[0] == 0 {
				continue
			}
			for _, b := range buf[:n] {
				switch b {
				case '\r', '\n':
					command := strings.TrimSpace(line.String())
					line.Reset()
					if err := writeTerminal(client, "\r\n"); err != nil {
						return err
					}
					if strings.EqualFold(command, "exit") || strings.EqualFold(command, "quit") {
						return writeTerminal(client, "Bye.\r\n")
					}
					if command != "" {
						result, err := executeCommandRequest(client.Context(), s, sqldb.QueryRequest{Query: command})
						rc.Audit(commandAuditResult(err), commandAuditParams(command, result, err), err)
						if err != nil {
							if err := writeTerminal(client, terminalError(err)); err != nil {
								return err
							}
						} else if err := writeTerminal(client, formatTerminalResult(result)); err != nil {
							return err
						}
					}
					if err := writeTerminal(client, prompt); err != nil {
						return err
					}
				case 3:
					line.Reset()
					if err := writeTerminal(client, "^C\r\n"+prompt); err != nil {
						return err
					}
				case 4:
					return writeTerminal(client, "\r\nBye.\r\n")
				case 12:
					if err := writeTerminal(client, "\x1b[2J\x1b[H"+prompt+line.String()); err != nil {
						return err
					}
				case 8, 127:
					if line.Len() > 0 {
						current := line.String()
						line.Reset()
						line.WriteString(current[:len(current)-1])
						if err := writeTerminal(client, "\b \b"); err != nil {
							return err
						}
					}
				default:
					if b >= 0x20 && b != 0x7f {
						line.WriteByte(b)
						if _, err := client.Write([]byte{b}); err != nil {
							return err
						}
					}
				}
			}
		}
		if err != nil {
			if client.Context().Err() != nil || errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func commandStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := redisSession(rc)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(client)
	enc := json.NewEncoder(client)
	for {
		var req sqldb.QueryRequest
		if err := dec.Decode(&req); err != nil {
			if client.Context().Err() != nil || errors.Is(err, io.EOF) {
				return nil
			}
			if err := enc.Encode(map[string]any{"error": "Invalid Redis command request."}); err != nil {
				return err
			}
			continue
		}
		result, err := executeCommandRequest(client.Context(), s, req)
		rc.Audit(commandAuditResult(err), commandAuditParams(req.Query, result, err), err)
		if err != nil {
			payload := map[string]any{"error": err.Error()}
			var confirmErr confirmationError
			if errors.As(err, &confirmErr) {
				payload["requiresConfirmation"] = true
				payload["confirmMessage"] = "This Redis command can change data, configuration, or server state. Review it before running."
			}
			if err := enc.Encode(payload); err != nil {
				return err
			}
			continue
		}
		if err := enc.Encode(result); err != nil {
			return err
		}
	}
}

func completionRoute(*plugin.RequestContext) (any, error) {
	commands := []string{
		"GET", "SET", "DEL", "EXISTS", "EXPIRE", "TTL", "TYPE", "SCAN", "KEYS",
		"HGETALL", "HGET", "HSET", "HDEL", "LRANGE", "LPUSH", "RPUSH", "LPOP", "RPOP",
		"SMEMBERS", "SADD", "SREM", "ZRANGE", "ZADD", "ZREM", "XINFO", "XRANGE",
		"INFO", "CLIENT LIST", "PUBSUB CHANNELS", "PING",
	}
	items := make([]sqldb.CompletionItem, 0, len(commands))
	for _, command := range commands {
		items = append(items, sqldb.CompletionItem{Label: command, Type: "keyword"})
	}
	return items, nil
}

func executeCommandRequest(parent context.Context, s *Session, req sqldb.QueryRequest) (sqldb.QueryResult, error) {
	if err := s.ensureOpen(); err != nil {
		return sqldb.QueryResult{}, err
	}
	args, err := parseCommand(req.Query)
	if err != nil {
		return sqldb.QueryResult{}, err
	}
	if len(args) == 0 {
		return sqldb.QueryResult{}, fmt.Errorf("%w: command is empty", plugin.ErrInvalidInput)
	}
	command := strings.ToUpper(args[0])
	if s.opts.ReadOnly && !isReadOnlyCommand(args) {
		return sqldb.QueryResult{}, fmt.Errorf("%w: read-only mode blocks write commands", plugin.ErrForbidden)
	}
	if s.opts.RequireConfirm && !req.Confirm && isDestructiveCommand(args) {
		return sqldb.QueryResult{}, confirmationError{message: "command requires confirmation"}
	}
	ctx, cancel := commandContext(parent, s)
	defer cancel()
	start := time.Now()
	values := make([]any, len(args))
	for i, arg := range args {
		values[i] = arg
	}
	value, err := s.client.Do(ctx, values...).Result()
	if err != nil {
		return sqldb.QueryResult{}, redisErr(err)
	}
	columns, rows := commandRows(value)
	return sqldb.QueryResult{
		Columns:    columns,
		Rows:       rows,
		RowCount:   int64(len(rows)),
		ElapsedMS:  time.Since(start).Milliseconds(),
		Statement:  req.Query,
		CommandTag: command,
	}, nil
}

func keySummary(ctx context.Context, s *Session, key string) (keyEntry, error) {
	kind, err := s.client.Type(ctx, key).Result()
	if err != nil {
		return keyEntry{}, redisErr(err)
	}
	ttl, err := s.client.TTL(ctx, key).Result()
	if err != nil {
		return keyEntry{}, redisErr(err)
	}
	size, _ := keySize(ctx, s, key, kind)
	return keyEntry{Key: key, Type: kind, TTL: int64(ttl.Seconds()), Size: size}, nil
}

func keyValue(ctx context.Context, s *Session, key string) (keyDetail, error) {
	kind, err := s.client.Type(ctx, key).Result()
	if err != nil {
		return keyDetail{}, redisErr(err)
	}
	if kind == "none" {
		return keyDetail{}, plugin.ErrNotFound
	}
	ttl, err := s.client.TTL(ctx, key).Result()
	if err != nil {
		return keyDetail{}, redisErr(err)
	}
	encoding, _ := s.client.ObjectEncoding(ctx, key).Result()
	value, err := readValue(ctx, s, key, kind)
	if err != nil {
		return keyDetail{}, err
	}
	size, _ := keySize(ctx, s, key, kind)
	return keyDetail{Key: key, Type: kind, TTL: int64(ttl.Seconds()), Size: size, Encoding: encoding, Value: value}, nil
}

func readValue(ctx context.Context, s *Session, key, kind string) (any, error) {
	limit := int64(s.opts.ValueLimit)
	switch kind {
	case "string":
		return s.client.Get(ctx, key).Result()
	case "hash":
		return s.client.HGetAll(ctx, key).Result()
	case "list":
		return s.client.LRange(ctx, key, 0, limit-1).Result()
	case "set":
		values, err := s.client.SMembers(ctx, key).Result()
		sort.Strings(values)
		return values, err
	case "zset":
		values, err := s.client.ZRangeWithScores(ctx, key, 0, limit-1).Result()
		if err != nil {
			return nil, redisErr(err)
		}
		out := make([]map[string]any, 0, len(values))
		for _, v := range values {
			out = append(out, map[string]any{"member": fmt.Sprint(v.Member), "score": v.Score})
		}
		return out, nil
	case "stream":
		values, err := s.client.XRange(ctx, key, "-", "+").Result()
		if err != nil {
			return nil, redisErr(err)
		}
		if len(values) > s.opts.ValueLimit {
			values = values[:s.opts.ValueLimit]
		}
		out := make([]map[string]any, 0, len(values))
		for _, msg := range values {
			out = append(out, map[string]any{"id": msg.ID, "values": msg.Values})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: Redis key type %q is not supported by the key browser", plugin.ErrNotSupported, kind)
	}
}

func replaceValue(ctx context.Context, s *Session, key, kind string, value any) error {
	switch kind {
	case "string":
		if err := s.client.Set(ctx, key, stringValue(value), 0).Err(); err != nil {
			return redisErr(err)
		}
	case "hash":
		values, err := stringMapValue(value)
		if err != nil {
			return err
		}
		if err := s.client.Del(ctx, key).Err(); err != nil {
			return redisErr(err)
		}
		if len(values) > 0 {
			if err := s.client.HSet(ctx, key, values).Err(); err != nil {
				return redisErr(err)
			}
		}
	case "list":
		values, err := stringSliceValue(value)
		if err != nil {
			return err
		}
		if err := s.client.Del(ctx, key).Err(); err != nil {
			return redisErr(err)
		}
		if len(values) > 0 {
			if err := s.client.RPush(ctx, key, values).Err(); err != nil {
				return redisErr(err)
			}
		}
	case "set":
		values, err := stringSliceValue(value)
		if err != nil {
			return err
		}
		if err := s.client.Del(ctx, key).Err(); err != nil {
			return redisErr(err)
		}
		if len(values) > 0 {
			if err := s.client.SAdd(ctx, key, values).Err(); err != nil {
				return redisErr(err)
			}
		}
	case "zset":
		values, err := zsetValue(value)
		if err != nil {
			return err
		}
		if err := s.client.Del(ctx, key).Err(); err != nil {
			return redisErr(err)
		}
		if len(values) > 0 {
			if err := s.client.ZAdd(ctx, key, values...).Err(); err != nil {
				return redisErr(err)
			}
		}
	default:
		return fmt.Errorf("%w: Redis key type %q cannot be written from the key browser", plugin.ErrNotSupported, kind)
	}
	return nil
}

func keySize(ctx context.Context, s *Session, key, kind string) (int64, error) {
	switch kind {
	case "string":
		return s.client.StrLen(ctx, key).Result()
	case "hash":
		return s.client.HLen(ctx, key).Result()
	case "list":
		return s.client.LLen(ctx, key).Result()
	case "set":
		return s.client.SCard(ctx, key).Result()
	case "zset":
		return s.client.ZCard(ctx, key).Result()
	case "stream":
		return s.client.XLen(ctx, key).Result()
	default:
		return 0, nil
	}
}

func commandRows(value any) ([]string, [][]any) {
	normalized := normalizeRedisValue(value)
	switch v := normalized.(type) {
	case []any:
		rows := make([][]any, 0, len(v))
		for i, item := range v {
			rows = append(rows, []any{i, formatCell(item)})
		}
		return []string{"index", "value"}, rows
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		rows := make([][]any, 0, len(keys))
		for _, key := range keys {
			rows = append(rows, []any{key, formatCell(v[key])})
		}
		return []string{"key", "value"}, rows
	default:
		return []string{"value"}, [][]any{{formatCell(v)}}
	}
}

func normalizeRedisValue(value any) any {
	switch v := value.(type) {
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeRedisValue(item))
		}
		return out
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case map[any]any:
		out := map[string]any{}
		for key, item := range v {
			out[fmt.Sprint(key)] = normalizeRedisValue(item)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = normalizeRedisValue(item)
		}
		return out
	case []byte:
		return string(v)
	case nil:
		return nil
	default:
		return v
	}
}

func formatCell(value any) any {
	switch v := value.(type) {
	case []any, map[string]any:
		raw, err := json.Marshal(v)
		if err == nil {
			return string(raw)
		}
	}
	return value
}

func parseCommand(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	args := []string{}
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range input {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if b.Len() > 0 {
				args = append(args, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("%w: unterminated quoted argument", plugin.ErrInvalidInput)
	}
	if b.Len() > 0 {
		args = append(args, b.String())
	}
	return args, nil
}

func redisPrompt(s *Session) string {
	return fmt.Sprintf("redis:%d> ", s.opts.Database)
}

func writeTerminal(w io.Writer, text string) error {
	_, err := io.WriteString(w, text)
	return err
}

func terminalError(err error) string {
	var confirmErr confirmationError
	if errors.As(err, &confirmErr) {
		return "(error) command requires confirmation; use a non-destructive command or disable required confirmation for this connection\r\n"
	}
	return "(error) " + err.Error() + "\r\n"
}

func formatTerminalResult(result sqldb.QueryResult) string {
	if len(result.Rows) == 0 {
		return "(empty)\r\n"
	}
	var b strings.Builder
	for _, row := range result.Rows {
		if len(result.Columns) == 1 && len(row) == 1 {
			b.WriteString(formatTerminalValue(row[0]))
			b.WriteString("\r\n")
			continue
		}
		for i, value := range row {
			if i > 0 {
				b.WriteString("\t")
			}
			if i < len(result.Columns) {
				b.WriteString(result.Columns[i])
				b.WriteString(": ")
			}
			b.WriteString(formatTerminalValue(value))
		}
		b.WriteString("\r\n")
	}
	return b.String()
}

func formatTerminalValue(value any) string {
	if value == nil {
		return "(nil)"
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}

func stringMapValue(value any) (map[string]string, error) {
	if s, ok := value.(string); ok {
		var out map[string]string
		if err := json.Unmarshal([]byte(s), &out); err == nil {
			return out, nil
		}
		return parseLineMap(s), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: hash value must be a JSON object", plugin.ErrInvalidInput)
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%w: hash value must be a JSON object", plugin.ErrInvalidInput)
	}
	return out, nil
}

func stringSliceValue(value any) ([]string, error) {
	if s, ok := value.(string); ok {
		var out []string
		if err := json.Unmarshal([]byte(s), &out); err == nil {
			return out, nil
		}
		return splitLines(s), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: collection value must be a JSON array", plugin.ErrInvalidInput)
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%w: collection value must be a JSON array", plugin.ErrInvalidInput)
	}
	return out, nil
}

func zsetValue(value any) ([]redisclient.Z, error) {
	if s, ok := value.(string); ok {
		var entries []struct {
			Member string  `json:"member"`
			Score  float64 `json:"score"`
		}
		if err := json.Unmarshal([]byte(s), &entries); err == nil {
			out := make([]redisclient.Z, 0, len(entries))
			for _, entry := range entries {
				out = append(out, redisclient.Z{Member: entry.Member, Score: entry.Score})
			}
			return out, nil
		}
		var mapped map[string]float64
		if err := json.Unmarshal([]byte(s), &mapped); err == nil {
			return zsetFromMap(mapped), nil
		}
		return nil, fmt.Errorf("%w: sorted set value must be JSON array or object", plugin.ErrInvalidInput)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: sorted set value must be JSON", plugin.ErrInvalidInput)
	}
	return zsetValue(string(raw))
}

func zsetFromMap(mapped map[string]float64) []redisclient.Z {
	keys := make([]string, 0, len(mapped))
	for key := range mapped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]redisclient.Z, 0, len(keys))
	for _, key := range keys {
		out = append(out, redisclient.Z{Member: key, Score: mapped[key]})
	}
	return out
}

func parseLineMap(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range splitLines(raw) {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			key, value, _ = strings.Cut(line, ":")
		}
		key = strings.TrimSpace(key)
		if key != "" {
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

func splitLines(raw string) []string {
	lines := strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' })
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func parseInfo(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if ok {
			out[key] = value
		}
	}
	return out
}

func parseClientList(raw string) []map[string]any {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		row := map[string]any{}
		for _, field := range strings.Fields(line) {
			key, value, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			row[key] = numericString(value)
		}
		out = append(out, row)
	}
	return out
}

func pageRows(rc *plugin.RequestContext, rows []map[string]any) (plugin.Page[map[string]any], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[map[string]any]{}, err
	}
	rows = filterRows(rows, req.Filter["q"])
	sortRows(rows, req.Sort)
	total := len(rows)
	start, err := offsetCursor(req.Cursor)
	if err != nil {
		return plugin.Page[map[string]any]{}, err
	}
	if start > len(rows) {
		start = len(rows)
	}
	end := min(start+req.Limit, len(rows))
	next := ""
	if end < len(rows) {
		next = strconv.Itoa(end)
	}
	return plugin.Page[map[string]any]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

func filterRows(rows []map[string]any, q string) []map[string]any {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := rows[:0]
	for _, row := range rows {
		for _, value := range row {
			if strings.Contains(strings.ToLower(fmt.Sprint(value)), q) {
				out = append(out, row)
				break
			}
		}
	}
	return out
}

func sortRows(rows []map[string]any, keys []plugin.SortKey) {
	if len(keys) == 0 {
		return
	}
	key := keys[0]
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := fmt.Sprint(rows[i][key.Field]), fmt.Sprint(rows[j][key.Field])
		if key.Desc {
			return a > b
		}
		return a < b
	})
}

func scanCursor(raw string) (uint64, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: cursor must be a Redis SCAN cursor", plugin.ErrInvalidInput)
	}
	return n, nil
}

func offsetCursor(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: cursor must be an offset", plugin.ErrInvalidInput)
	}
	return n, nil
}

func numericString(value string) any {
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return n
	}
	return value
}

func normalizeType(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "string", "hash", "list", "set", "zset":
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return ""
	}
}

func ensureWritable(s *Session) error {
	if s.opts.ReadOnly {
		return fmt.Errorf("%w: read-only mode blocks write operations", plugin.ErrForbidden)
	}
	return nil
}

func isReadOnlyCommand(args []string) bool {
	if len(args) == 0 {
		return true
	}
	command := strings.ToUpper(args[0])
	switch command {
	case "GET", "MGET", "EXISTS", "TTL", "PTTL", "TYPE", "SCAN", "SSCAN", "HSCAN", "ZSCAN", "KEYS",
		"HGET", "HGETALL", "HMGET", "HLEN", "HKEYS", "HVALS", "LRANGE", "LLEN", "SMEMBERS", "SCARD",
		"ZRANGE", "ZREVRANGE", "ZCARD", "ZSCORE", "XLEN", "XRANGE", "XREVRANGE", "INFO", "CLIENT", "PUBSUB",
		"PING", "ECHO", "DBSIZE", "MEMORY", "OBJECT":
		if command == "CLIENT" {
			return len(args) > 1 && strings.EqualFold(args[1], "LIST")
		}
		if command == "MEMORY" {
			return len(args) > 1 && strings.EqualFold(args[1], "USAGE")
		}
		if command == "OBJECT" {
			return len(args) > 1 && (strings.EqualFold(args[1], "ENCODING") || strings.EqualFold(args[1], "IDLETIME") || strings.EqualFold(args[1], "FREQ"))
		}
		return true
	default:
		return false
	}
}

func isDestructiveCommand(args []string) bool {
	return !isReadOnlyCommand(args)
}

func commandAuditResult(err error) models.AuditResult {
	if err == nil {
		return models.AuditAllowed
	}
	var confirmErr confirmationError
	if errors.As(err, &confirmErr) {
		return models.AuditDenied
	}
	if errors.Is(err, plugin.ErrForbidden) {
		return models.AuditDenied
	}
	return models.AuditError
}

func commandAuditParams(command string, result sqldb.QueryResult, err error) map[string]string {
	params := map[string]string{
		"command": commandName(command),
		"hash":    sqldb.QueryHash(command),
	}
	if result.ElapsedMS > 0 {
		params["elapsedMs"] = strconv.FormatInt(result.ElapsedMS, 10)
	}
	if err != nil {
		params["error"] = err.Error()
	}
	return params
}

func commandName(command string) string {
	args, err := parseCommand(command)
	if err != nil || len(args) == 0 {
		return ""
	}
	return strings.ToUpper(args[0])
}

func commandContext(parent context.Context, s *Session) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, s.opts.Timeout)
}

func redisErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, redisclient.Nil) {
		return plugin.ErrNotFound
	}
	return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
}

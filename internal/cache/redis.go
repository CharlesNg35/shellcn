package cache

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RedisConfig captures the minimal connection parameters required by the lightweight Redis client.
type RedisConfig struct {
	Address  string
	Username string
	Password string
	DB       int
	TLS      bool
	Timeout  time.Duration
}

const defaultRedisTimeout = 5 * time.Second
const redisKeyPrefix = "shellcn:"

// RedisClient implements a small subset of the Redis protocol required by the ShellCN backend.
// It supports AUTH, SELECT, INCR, PEXPIRE, PTTL, GET, SET (with PX) and DEL commands.
// The implementation is intentionally simple and maintains a single connection guarded by a mutex.
type RedisClient struct {
	cfg    RedisConfig
	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
}

// NewRedisClient creates a new Redis client. It eagerly establishes the connection so that
// misconfiguration is surfaced during application startup.
func NewRedisClient(cfg RedisConfig) (*RedisClient, error) {
	cfg.Address = strings.TrimSpace(cfg.Address)
	if cfg.Address == "" {
		return nil, errors.New("redis: address is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultRedisTimeout
	}

	client := &RedisClient{cfg: cfg}
	if err := client.ensureConnection(context.Background()); err != nil {
		return nil, err
	}
	return client, nil
}

// Close closes the underlying network connection.
func (c *RedisClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.reader = nil
		return err
	}
	return nil
}

// IncrementWithTTL increments the supplied key and ensures the TTL is set to the requested window.
// It returns the current count and the remaining time-to-live.
func (c *RedisClient) IncrementWithTTL(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	prefixedKey := c.prefixed(key)
	count, err := c.doInt(ctx, "INCR", prefixedKey)
	if err != nil {
		return 0, 0, err
	}

	if count == 1 {
		if _, err := c.doInt(ctx, "PEXPIRE", prefixedKey, formatMillis(window)); err != nil {
			return 0, 0, err
		}
	}

	ttlMillis, err := c.doInt(ctx, "PTTL", prefixedKey)
	if err != nil {
		return count, window, nil
	}

	if ttlMillis < 0 {
		return count, window, nil
	}
	return count, time.Duration(ttlMillis) * time.Millisecond, nil
}

// Set stores a value with PX expiry semantics.
func (c *RedisClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	prefixedKey := c.prefixed(key)
	_, err := c.doSimple(ctx, "SET", prefixedKey, string(value), "PX", formatMillis(ttl))
	return err
}

// Get retrieves the value associated with a key.
func (c *RedisClient) Get(ctx context.Context, key string) ([]byte, bool, error) {
	prefixedKey := c.prefixed(key)
	resp, err := c.do(ctx, "GET", prefixedKey)
	if err != nil {
		return nil, false, err
	}

	switch v := resp.(type) {
	case nil:
		return nil, false, nil
	case []byte:
		return v, true, nil
	default:
		return nil, false, fmt.Errorf("redis: unexpected response type %T", v)
	}
}

// Delete removes one or more keys, ignoring missing keys.
func (c *RedisClient) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	args := make([]string, 0, len(keys)+1)
	args = append(args, "DEL")
	for _, key := range keys {
		args = append(args, c.prefixed(key))
	}
	_, err := c.do(ctx, args...)
	return err
}

func (c *RedisClient) prefixed(key string) string {
	normalized := normalizeKey(key)
	if strings.HasPrefix(normalized, redisKeyPrefix) {
		return normalized
	}
	return normalizeKey(redisKeyPrefix + normalized)
}

func (c *RedisClient) doSimple(ctx context.Context, command string, args ...string) (string, error) {
	resp, err := c.do(ctx, append([]string{command}, args...)...)
	if err != nil {
		return "", err
	}
	switch v := resp.(type) {
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("redis: unexpected simple response %T", v)
	}
}

func (c *RedisClient) doInt(ctx context.Context, command string, args ...string) (int64, error) {
	resp, err := c.do(ctx, append([]string{command}, args...)...)
	if err != nil {
		return 0, err
	}
	switch v := resp.(type) {
	case int64:
		return v, nil
	case string:
		i, convErr := strconv.ParseInt(v, 10, 64)
		if convErr != nil {
			return 0, convErr
		}
		return i, nil
	default:
		return 0, fmt.Errorf("redis: unexpected integer response %T", v)
	}
}

func (c *RedisClient) do(ctx context.Context, args ...string) (interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConnectionLocked(ctx); err != nil {
		return nil, err
	}

	deadline := deadlineFromContext(ctx, c.cfg.Timeout)
	if err := c.conn.SetDeadline(deadline); err != nil {
		c.resetLocked()
		return nil, err
	}

	if err := writeCommand(c.conn, args); err != nil {
		c.resetLocked()
		return nil, err
	}

	resp, err := readResponse(c.reader)
	if err != nil {
		c.resetLocked()
		return nil, err
	}

	return resp, nil
}

func (c *RedisClient) ensureConnection(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.ensureConnectionLocked(ctx)
}

func (c *RedisClient) ensureConnectionLocked(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	var (
		conn net.Conn
		err  error
	)

	if c.cfg.TLS {
		dialer := &tls.Dialer{NetDialer: &net.Dialer{}}
		conn, err = dialer.DialContext(ctx, "tcp", c.cfg.Address)
	} else {
		dialer := &net.Dialer{}
		conn, err = dialer.DialContext(ctx, "tcp", c.cfg.Address)
	}
	if err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	deadline := deadlineFromContext(ctx, c.cfg.Timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return err
	}

	if c.cfg.Password != "" || c.cfg.Username != "" {
		authArgs := []string{"AUTH"}
		if c.cfg.Username != "" {
			authArgs = append(authArgs, c.cfg.Username, c.cfg.Password)
		} else {
			authArgs = append(authArgs, c.cfg.Password)
		}
		if err := writeCommand(conn, authArgs); err != nil {
			conn.Close()
			return err
		}
		if resp, err := readResponse(reader); err != nil {
			conn.Close()
			return err
		} else if str, ok := resp.(string); !ok || !strings.EqualFold(str, "OK") {
			conn.Close()
			return fmt.Errorf("redis: AUTH failed: %v", resp)
		}
	}

	if c.cfg.DB > 0 {
		if err := writeCommand(conn, []string{"SELECT", strconv.Itoa(c.cfg.DB)}); err != nil {
			conn.Close()
			return err
		}
		if resp, err := readResponse(reader); err != nil {
			conn.Close()
			return err
		} else if str, ok := resp.(string); !ok || !strings.EqualFold(str, "OK") {
			conn.Close()
			return fmt.Errorf("redis: SELECT failed: %v", resp)
		}
	}

	// Clear deadlines; runtime commands will set per-call deadlines
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return err
	}

	c.conn = conn
	c.reader = reader
	return nil
}

func (c *RedisClient) resetLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.reader = nil
}

func deadlineFromContext(ctx context.Context, fallback time.Duration) time.Time {
	if deadline, ok := ctx.Deadline(); ok {
		return deadline
	}
	return time.Now().Add(fallback)
}

func writeCommand(conn net.Conn, args []string) error {
	builder := strings.Builder{}
	builder.Grow(1 + len(args)*4) // rough estimate
	builder.WriteByte('*')
	builder.WriteString(strconv.Itoa(len(args)))
	builder.WriteString("\r\n")
	for _, arg := range args {
		builder.WriteByte('$')
		builder.WriteString(strconv.Itoa(len(arg)))
		builder.WriteString("\r\n")
		builder.WriteString(arg)
		builder.WriteString("\r\n")
	}
	_, err := conn.Write([]byte(builder.String()))
	return err
}

func readResponse(r *bufio.Reader) (interface{}, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch prefix {
	case '+':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		return line, nil
	case '-':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(line)
	case ':':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		n, convErr := strconv.ParseInt(line, 10, 64)
		if convErr != nil {
			return nil, convErr
		}
		return n, nil
	case '$':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		length, convErr := strconv.Atoi(line)
		if convErr != nil {
			return nil, convErr
		}
		if length == -1 {
			return nil, nil
		}
		buf := make([]byte, length)
		if _, err := r.Read(buf); err != nil {
			return nil, err
		}
		if err := consumeCRLF(r); err != nil {
			return nil, err
		}
		return buf, nil
	case '*':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		count, convErr := strconv.Atoi(line)
		if convErr != nil {
			return nil, convErr
		}
		items := make([]interface{}, count)
		for i := 0; i < count; i++ {
			item, err := readResponse(r)
			if err != nil {
				return nil, err
			}
			items[i] = item
		}
		return items, nil
	default:
		return nil, fmt.Errorf("redis: unexpected prefix %q", prefix)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	return strings.TrimSuffix(line, "\r"), nil
}

func consumeCRLF(r *bufio.Reader) error {
	first, err := r.ReadByte()
	if err != nil {
		return err
	}
	second, err := r.ReadByte()
	if err != nil {
		return err
	}
	if first != '\r' || second != '\n' {
		return errors.New("redis: expected CRLF")
	}
	return nil
}

func normalizeKey(key string) string {
	if key == "" {
		return key
	}
	var builder strings.Builder
	builder.Grow(len(key))
	prevColon := false
	for i := 0; i < len(key); i++ {
		ch := key[i]
		if ch == ':' {
			if prevColon {
				continue
			}
			prevColon = true
		} else {
			prevColon = false
		}
		builder.WriteByte(ch)
	}
	return builder.String()
}

func formatMillis(duration time.Duration) string {
	if duration <= 0 {
		return "0"
	}
	return strconv.FormatInt(duration.Milliseconds(), 10)
}

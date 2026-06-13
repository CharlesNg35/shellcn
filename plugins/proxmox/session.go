package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	pmox "github.com/luthermonson/go-proxmox"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Session holds the per-connection Proxmox VE client. REST traffic flows through
// the go-proxmox client (wired to cfg.Net via a custom http.Client); the console
// websocket is dialed separately so it shares the same transport.
type Session struct {
	client *pmox.Client
	httpc  *http.Client
	wsBase string
	apply  func(http.Header)
}

func connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseConnectOptions(cfg)
	if err != nil {
		return nil, err
	}

	httpc := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			DialContext:         cfg.Net.DialContext,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: !opts.VerifyTLS}, //nolint:gosec // self-signed PVE certs are the norm; the operator opts in to verification.
			TLSHandshakeTimeout: 15 * time.Second,
		},
	}
	baseURL := "https://" + opts.Addr + "/api2/json"

	clientOpts := []pmox.Option{pmox.WithHTTPClient(httpc)}
	var apply func(http.Header)
	switch opts.Method {
	case authToken:
		clientOpts = append(clientOpts, pmox.WithAPIToken(opts.TokenID, opts.TokenSecret))
		header := "PVEAPIToken=" + opts.TokenID + "=" + opts.TokenSecret
		apply = func(h http.Header) { h.Set("Authorization", header) }
	case authPassword:
		ticket, csrf, err := loginTicket(ctx, httpc, baseURL, opts.Username, opts.Password)
		if err != nil {
			return nil, err
		}
		clientOpts = append(clientOpts, pmox.WithSession(ticket, csrf))
		cookie := "PVEAuthCookie=" + ticket
		apply = func(h http.Header) { h.Set("Cookie", cookie) }
	}

	s := &Session{
		client: pmox.NewClient(baseURL, clientOpts...),
		httpc:  httpc,
		wsBase: "wss://" + opts.Addr + "/api2/json",
		apply:  apply,
	}
	if err := s.HealthCheck(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) HealthCheck(ctx context.Context) error {
	if _, err := s.client.Version(ctx); err != nil {
		return fmt.Errorf("%w: reach proxmox api: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

func (s *Session) Close() error { return nil }

func (s *Session) OpenChannel(ctx context.Context, req plugin.ChannelRequest) (plugin.Channel, error) {
	switch req.Kind {
	case plugin.StreamDesktop:
		return s.openVMConsole(ctx, req.Params["node"], req.Params["vmid"])
	case plugin.StreamTerminal:
		return s.openTerminal(ctx, req.Params)
	default:
		return nil, plugin.ErrNotSupported
	}
}

// Unwrap recovers the concrete Proxmox session from a (possibly wrapped) plugin
// session, mirroring the helper the other plugins expose for their route code.
func Unwrap(sess plugin.Session) (*Session, error) {
	if s, ok := sess.(*Session); ok {
		return s, nil
	}
	type sessionGetter interface{ Session() plugin.Session }
	if h, ok := sess.(sessionGetter); ok {
		if s, ok := h.Session().(*Session); ok {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: Proxmox session unavailable", plugin.ErrUnavailable)
}

// loginTicket exchanges username/password for a PVE ticket + CSRF token. The
// ticket doubles as the websocket auth cookie, so the gateway owns it directly
// rather than relying on the client's lazily-created session.
func loginTicket(ctx context.Context, httpc *http.Client, baseURL, username, password string) (ticket, csrf string, err error) {
	form := url.Values{"username": {username}, "password": {password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/access/ticket", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("%w: build login request: %v", plugin.ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpc.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: proxmox login: %v", plugin.ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("%w: proxmox login failed", plugin.ErrUnauthorized)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%w: proxmox login returned %s", plugin.ErrUnavailable, resp.Status)
	}
	var out struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("%w: decode login response: %v", plugin.ErrUnavailable, err)
	}
	if out.Data.Ticket == "" {
		return "", "", fmt.Errorf("%w: proxmox login returned no ticket", plugin.ErrUnauthorized)
	}
	return out.Data.Ticket, out.Data.CSRFPreventionToken, nil
}

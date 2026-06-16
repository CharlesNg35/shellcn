package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/charlesng35/shellcn/internal/livelease"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const leaseProxyHeader = "X-ShellCN-Lease-Proxy"

var leaseProbeTimeout = 750 * time.Millisecond

func (s *Server) proxyIfRemoteLeaseHolder(w http.ResponseWriter, r *http.Request, conn models.Connection, userID string) bool {
	ref, ok, err := s.remoteLeaseHolder(r.Context(), conn, userID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return true
	}
	if !ok {
		return false
	}
	if r.Header.Get(leaseProxyHeader) != "" {
		writeError(w, s.deps.Logger, fmt.Errorf("%w: live state lease is held by another instance", plugin.ErrUnavailable))
		return true
	}
	target, err := s.resolveLeaseProxyTarget(r.Context(), ref)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return true
	}
	s.proxyToLeaseHolder(w, r, target)
	return true
}

func (s *Server) resolveLeaseProxyTarget(ctx context.Context, ref livelease.LeaseRef) (*url.URL, error) {
	candidates := ref.InternalURLCandidates()
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: lease holder is not reachable", plugin.ErrUnavailable)
	}
	var lastErr error
	for _, candidate := range candidates {
		target, err := url.Parse(candidate)
		if err != nil || target.Scheme == "" || target.Host == "" {
			lastErr = fmt.Errorf("invalid lease URL %q", candidate)
			continue
		}
		if err := probeLeaseHolder(ctx, target); err != nil {
			lastErr = err
			continue
		}
		if candidate != ref.Instance.PreferredInternalURL() && s.deps.Leases != nil {
			if err := s.deps.Leases.PreferInternalURL(context.WithoutCancel(ctx), ref, candidate); err != nil && s.deps.Logger != nil {
				s.deps.Logger.Warn("prefer live-state lease URL failed", "ref", ref.Key, "url", candidate, "err", err)
			}
		}
		return target, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate lease URL")
	}
	return nil, fmt.Errorf("%w: lease holder is not reachable: %v", plugin.ErrUnavailable, lastErr)
}

func probeLeaseHolder(ctx context.Context, target *url.URL) error {
	probeCtx, cancel := context.WithTimeout(ctx, leaseProbeTimeout)
	defer cancel()
	u := *target
	u.Path = "/healthz"
	u.RawPath = ""
	u.RawQuery = ""
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lease holder health returned %s", resp.Status)
	}
	return nil
}

func (s *Server) proxyToLeaseHolder(w http.ResponseWriter, r *http.Request, target *url.URL) {
	if target == nil {
		writeError(w, s.deps.Logger, fmt.Errorf("%w: lease holder is not reachable", plugin.ErrUnavailable))
		return
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			pr.Out.URL.Path = pr.In.URL.Path
			pr.Out.URL.RawPath = pr.In.URL.RawPath
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
			pr.Out.Host = pr.In.Host
			pr.Out.Header.Set(leaseProxyHeader, s.deps.Instance.ID)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, s.deps.Logger, fmt.Errorf("%w: lease holder proxy failed: %v", plugin.ErrUnavailable, err))
		},
	}
	proxy.ServeHTTP(w, r)
}

func (s *Server) remoteLeaseHolder(ctx context.Context, conn models.Connection, userID string) (livelease.LeaseRef, bool, error) {
	if s.deps.Leases == nil {
		return livelease.LeaseRef{}, false, nil
	}
	if ref, ok, err := s.deps.Leases.Get(ctx, livelease.SessionLeaseKey(conn.ID, userID)); err != nil {
		return livelease.LeaseRef{}, false, err
	} else if ok {
		return ref, !ref.IsLocal(s.deps.Instance), nil
	}
	if conn.Transport != string(plugin.TransportAgent) {
		return livelease.LeaseRef{}, false, nil
	}
	ref, ok, err := s.deps.Leases.Get(ctx, livelease.AgentLeaseKey(conn.ID))
	if err != nil || !ok {
		return livelease.LeaseRef{}, false, err
	}
	return ref, !ref.IsLocal(s.deps.Instance), nil
}

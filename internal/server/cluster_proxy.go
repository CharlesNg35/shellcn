package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/charlesng35/shellcn/internal/cluster"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const clusterProxyHeader = "X-ShellCN-Cluster-Proxy"

var ownerProbeTimeout = 750 * time.Millisecond

func (s *Server) proxyIfRemoteOwner(w http.ResponseWriter, r *http.Request, conn models.Connection, userID string) bool {
	owner, ok, err := s.remoteOwner(r.Context(), conn, userID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return true
	}
	if !ok {
		return false
	}
	if r.Header.Get(clusterProxyHeader) != "" {
		writeError(w, s.deps.Logger, fmt.Errorf("%w: live state is owned by another instance", plugin.ErrUnavailable))
		return true
	}
	target, err := s.resolveOwnerProxyTarget(r.Context(), owner)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return true
	}
	s.proxyToOwner(w, r, target)
	return true
}

func (s *Server) resolveOwnerProxyTarget(ctx context.Context, owner cluster.OwnerRef) (*url.URL, error) {
	candidates := owner.InternalURLCandidates()
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: owning instance is not reachable", plugin.ErrUnavailable)
	}
	var lastErr error
	for _, candidate := range candidates {
		target, err := url.Parse(candidate)
		if err != nil || target.Scheme == "" || target.Host == "" {
			lastErr = fmt.Errorf("invalid owner URL %q", candidate)
			continue
		}
		if err := probeOwner(ctx, target); err != nil {
			lastErr = err
			continue
		}
		if candidate != owner.Instance.PreferredInternalURL() && s.deps.Owners != nil {
			if err := s.deps.Owners.PreferInternalURL(context.WithoutCancel(ctx), owner, candidate); err != nil && s.deps.Logger != nil {
				s.deps.Logger.Warn("prefer cluster owner URL failed", "owner", owner.Key, "url", candidate, "err", err)
			}
		}
		return target, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate owner URL")
	}
	return nil, fmt.Errorf("%w: owning instance is not reachable: %v", plugin.ErrUnavailable, lastErr)
}

func probeOwner(ctx context.Context, target *url.URL) error {
	probeCtx, cancel := context.WithTimeout(ctx, ownerProbeTimeout)
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
		return fmt.Errorf("owner health returned %s", resp.Status)
	}
	return nil
}

func (s *Server) proxyToOwner(w http.ResponseWriter, r *http.Request, target *url.URL) {
	if target == nil {
		writeError(w, s.deps.Logger, fmt.Errorf("%w: owning instance is not reachable", plugin.ErrUnavailable))
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
			pr.Out.Header.Set(clusterProxyHeader, s.deps.Instance.ID)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, s.deps.Logger, fmt.Errorf("%w: owning instance proxy failed: %v", plugin.ErrUnavailable, err))
		},
	}
	proxy.ServeHTTP(w, r)
}

func (s *Server) remoteOwner(ctx context.Context, conn models.Connection, userID string) (cluster.OwnerRef, bool, error) {
	if s.deps.Owners == nil {
		return cluster.OwnerRef{}, false, nil
	}
	if owner, ok, err := s.deps.Owners.Get(ctx, cluster.SessionOwnerKey(conn.ID, userID)); err != nil {
		return cluster.OwnerRef{}, false, err
	} else if ok {
		return owner, !owner.IsLocal(s.deps.Instance), nil
	}
	if conn.Transport != string(plugin.TransportAgent) {
		return cluster.OwnerRef{}, false, nil
	}
	owner, ok, err := s.deps.Owners.Get(ctx, cluster.AgentOwnerKey(conn.ID))
	if err != nil || !ok {
		return cluster.OwnerRef{}, false, err
	}
	return owner, !owner.IsLocal(s.deps.Instance), nil
}

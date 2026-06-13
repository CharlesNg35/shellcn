package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/charlesng35/shellcn/internal/cluster"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const clusterProxyHeader = "X-ShellCN-Cluster-Proxy"

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
	if owner.Instance.InternalURL == "" {
		writeError(w, s.deps.Logger, fmt.Errorf("%w: owning instance is not reachable", plugin.ErrUnavailable))
		return true
	}
	target, err := url.Parse(owner.Instance.InternalURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		writeError(w, s.deps.Logger, fmt.Errorf("%w: owning instance has an invalid address", plugin.ErrUnavailable))
		return true
	}
	s.proxyToOwner(w, r, target)
	return true
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

func (s *Server) proxyToOwner(w http.ResponseWriter, r *http.Request, target *url.URL) {
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

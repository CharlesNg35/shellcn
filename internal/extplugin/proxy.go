package extplugin

import (
	"io"
	"net/http"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/grpcplugin"
)

// ServeHTTPProxy reverse-proxies a browser request to the plugin. It hijacks the
// browser connection and bridges raw bytes to a brokered conn the plugin serves,
// so redirects, assets, and WebSocket upgrades pass through unchanged.
func (s *grpcSession) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	client, broker := s.ref.get()
	ref, err := client.ServeHTTPProxy(r.Context(), &pluginv1.ProxyRequest{SessionId: s.id, Path: r.URL.Path})
	if err != nil {
		http.Error(w, "plugin proxy unavailable", http.StatusBadGateway)
		return
	}
	conn, err := grpcplugin.DialConn(broker, ref.GetBrokerId())
	if err != nil {
		http.Error(w, "plugin proxy unavailable", http.StatusBadGateway)
		return
	}
	defer func() { _ = conn.Close() }()

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "proxy requires a hijackable connection", http.StatusInternalServerError)
		return
	}
	browser, brw, err := hj.Hijack()
	if err != nil {
		return
	}
	defer func() { _ = browser.Close() }()

	r.RequestURI = ""
	if err := r.Write(conn); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(conn, brw); done <- struct{}{} }()
	go func() { _, _ = io.Copy(browser, conn); done <- struct{}{} }()
	<-done
	_ = conn.Close()
	_ = browser.Close()
	<-done
}

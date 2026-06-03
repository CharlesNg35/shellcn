package extplugin

import (
	"io"
	"net"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// stream bridges the browser ClientStream to the plugin's streaming handler over
// a brokered conn, so the core stays the byte-pump (and recorder) in the middle.
func (g *grpcPlugin) stream(rc *plugin.RequestContext, browser plugin.ClientStream, routeID string) error {
	sess, ok := rc.Session.(*grpcSession)
	if !ok {
		return plugin.ErrUnavailable
	}
	client, broker := g.ref.get()
	ref, err := client.OpenStream(rc.Ctx, &pluginv1.StreamStart{
		SessionId: sess.id, RouteId: routeID, Params: rc.Params(), User: wireUser(rc.User),
	})
	if err != nil {
		return grpcplugin.ErrorFromStatus(err)
	}
	conn, err := grpcplugin.DialConn(broker, ref.GetBrokerId())
	if err != nil {
		return err
	}
	bridgeStream(browser, conn)
	return nil
}

// bridgeStream copies bytes both ways between the browser stream and the brokered
// conn, tearing down when the browser disconnects.
func bridgeStream(browser plugin.ClientStream, conn net.Conn) {
	go func() {
		<-browser.Context().Done()
		_ = conn.Close()
	}()
	done := make(chan struct{}, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go cp(conn, browser)
	go cp(browser, conn)
	<-done
	_ = conn.Close()
	_ = browser.Close()
	<-done
}

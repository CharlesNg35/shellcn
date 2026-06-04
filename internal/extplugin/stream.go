package extplugin

import (
	"fmt"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// stream bridges the browser ClientStream to the plugin's streaming handler over
// a brokered conn, so the core stays the byte-pump (and recorder) in the middle.
func (g *grpcPlugin) stream(rc *plugin.RequestContext, browser plugin.ClientStream, routeID string) error {
	sess, ok := sessionOf(rc.Session)
	if !ok {
		return fmt.Errorf("%w: request session does not belong to this external plugin", plugin.ErrUnavailable)
	}
	client, broker := g.ref.get()
	ref, err := client.OpenStream(rc.Ctx, &pluginv1.StreamStart{
		SessionId: sess.id, RouteId: routeID, Params: rc.Params(), User: wireUser(rc.User),
		ProxyPrefix: rc.ProxyPrefix(),
	})
	if err != nil {
		return grpcplugin.ErrorFromStatus(err)
	}
	conn, err := grpcplugin.DialConn(broker, ref.GetBrokerId())
	if err != nil {
		return err
	}
	// Close the conn if the browser context is cancelled while a copy is blocked;
	// the watcher exits when the bridge completes so it can't leak.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-browser.Context().Done():
			_ = conn.Close()
		case <-stop:
		}
	}()
	grpcplugin.Bridge(browser, conn)
	return nil
}

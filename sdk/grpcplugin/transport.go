package grpcplugin

import (
	"context"
	"net"
	"net/http"

	goplugin "github.com/hashicorp/go-plugin"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
)

// brokerTransport is the plugin-side NetTransport: it dials targets through the
// core's Host service and returns the brokered byte stream as a net.Conn. Like a
// direct transport it offers no injected L7 client; a plugin's HTTP client uses
// DialContext.
type brokerTransport struct {
	broker *goplugin.GRPCBroker
	host   pluginv1.HostClient
}

func newBrokerTransport(broker *goplugin.GRPCBroker, host pluginv1.HostClient) *brokerTransport {
	return &brokerTransport{broker: broker, host: host}
}

func (t *brokerTransport) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	ref, err := t.host.DialTarget(ctx, &pluginv1.DialRequest{Network: network, Address: address})
	if err != nil {
		return nil, err
	}
	cc, err := t.broker.Dial(ref.GetBrokerId())
	if err != nil {
		return nil, err
	}
	streamCtx, cancel := context.WithCancel(context.Background())
	stream, err := pluginv1.NewConnClient(cc).Pipe(streamCtx)
	if err != nil {
		cancel()
		_ = cc.Close()
		return nil, err
	}
	return newStreamConn(stream, cancel), nil
}

func (*brokerTransport) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

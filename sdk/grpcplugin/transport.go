package grpcplugin

import (
	"context"
	"net"
	"net/http"

	goplugin "github.com/hashicorp/go-plugin"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
)

// brokerTransport is the plugin-side NetTransport: it reaches targets through the
// core's Host service. DialContext brokers an L4 conn; HTTP() brokers conns to
// the core's L7 reverse proxy when the connection has an L7 transport.
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
	return t.dialPipe(ref)
}

func (t *brokerTransport) HTTP() (string, http.RoundTripper, bool) {
	ep, err := t.host.HTTPProxyEndpoint(context.Background(), &pluginv1.SessionHandle{})
	if err != nil || ep.GetBaseUrl() == "" {
		return "", nil, false
	}
	rt := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			ref, err := t.host.OpenHTTPConn(ctx, &pluginv1.SessionHandle{})
			if err != nil {
				return nil, err
			}
			return t.dialPipe(ref)
		},
	}
	return ep.GetBaseUrl(), rt, true
}

func (t *brokerTransport) dialPipe(ref *pluginv1.BrokerRef) (net.Conn, error) {
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

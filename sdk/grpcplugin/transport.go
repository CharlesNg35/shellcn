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
	return DialConn(t.broker, ref.GetBrokerId())
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
			return DialConn(t.broker, ref.GetBrokerId())
		},
	}
	return ep.GetBaseUrl(), rt, true
}

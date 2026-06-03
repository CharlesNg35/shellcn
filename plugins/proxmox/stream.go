package proxmox

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/charlesng35/shellcn/plugins/shared/rfb"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const metricsInterval = 2 * time.Second

// guestMetrics streams live CPU/memory for a VM or container by polling its
// status endpoint and emitting `{cpu, mem}` percentage frames for the metrics
// panel.
func guestMetrics(kind string) plugin.StreamHandler {
	return func(rc *plugin.RequestContext, client plugin.ClientStream) error {
		path := fmt.Sprintf("/nodes/%s/%s/%s/status/current", rc.Param("node"), kind, rc.Param("vmid"))
		return metricsLoop(rc, client, path)
	}
}

func nodeMetrics(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return metricsLoop(rc, client, "/nodes/"+rc.Param("node")+"/status")
}

func metricsLoop(rc *plugin.RequestContext, client plugin.ClientStream, statusPath string) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(metricsInterval)
	defer ticker.Stop()
	for {
		if status, err := s.object(rc.Ctx, statusPath); err == nil {
			if err := enc.Encode(metricFrame(status)); err != nil {
				return err
			}
		}
		select {
		case <-client.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}

func metricFrame(status row) map[string]any {
	cpu := round1(numFloat(status["cpu"]) * 100)
	var memPct float64
	if maxmem := numFloat(status["maxmem"]); maxmem > 0 {
		memPct = numFloat(status["mem"]) / maxmem * 100
	} else if mem, ok := status["memory"].(map[string]any); ok {
		if total := numFloat(mem["total"]); total > 0 {
			memPct = numFloat(mem["used"]) / total * 100
		}
	}
	return map[string]any{"cpu": cpu, "mem": round1(memPct)}
}

// vmConsole splices the authenticated upstream RFB stream to the browser's noVNC
// client: gateway-side Security-None handshake, the upstream ServerInit, then a
// raw byte pipe both ways.
func vmConsole(rc *plugin.RequestContext, client plugin.ClientStream) error {
	ch, err := rc.Session.OpenChannel(rc.Ctx, plugin.ChannelRequest{
		Kind:   plugin.StreamDesktop,
		Params: map[string]string{"node": rc.Param("node"), "vmid": rc.Param("vmid")},
	})
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	si, ok := ch.(interface{ ServerInit() []byte })
	if !ok {
		return fmt.Errorf("%w: console channel missing server init", plugin.ErrUnavailable)
	}
	if err := rfb.ServerHandshakeNone(client); err != nil {
		return err
	}
	if _, err := client.Write(si.ServerInit()); err != nil {
		return err
	}
	return splice(client, ch)
}

// terminalConsole bridges an xterm panel to an LXC or node shell over termproxy.
func terminalConsole(kind string) plugin.StreamHandler {
	return func(rc *plugin.RequestContext, client plugin.ClientStream) error {
		params := map[string]string{"kind": kind, "node": rc.Param("node")}
		if kind == "lxc" {
			params["vmid"] = rc.Param("vmid")
		}
		ch, err := rc.Session.OpenChannel(rc.Ctx, plugin.ChannelRequest{Kind: plugin.StreamTerminal, Params: params})
		if err != nil {
			return err
		}
		defer func() { _ = ch.Close() }()

		errc := make(chan error, 2)
		go func() { _, err := io.Copy(client, ch); errc <- err }()
		go func() { errc <- plugin.CopyTerminalInput(ch, client) }()
		select {
		case <-client.Context().Done():
			return nil
		case err := <-errc:
			return ignoreEOF(err)
		}
	}
}

func splice(client plugin.ClientStream, ch plugin.Channel) error {
	errc := make(chan error, 2)
	go func() { _, err := io.Copy(client, ch); errc <- err }()
	go func() { _, err := io.Copy(ch, client); errc <- err }()
	select {
	case <-client.Context().Done():
		return nil
	case err := <-errc:
		return ignoreEOF(err)
	}
}

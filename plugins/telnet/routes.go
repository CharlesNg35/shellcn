package telnet

import (
	"encoding/json"
	"io"
	"net/url"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func terminalSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Terminal", Fields: []plugin.Field{
		{Key: "cols", Label: "Columns", Type: plugin.FieldNumber},
		{Key: "rows", Label: "Rows", Type: plugin.FieldNumber},
	}}}}
}

func shell(rc *plugin.RequestContext, client plugin.ClientStream) error {
	ch, err := rc.Session.OpenChannel(rc.Ctx, plugin.ChannelRequest{Kind: plugin.StreamTerminal, Params: terminalParams(rc.Query())})
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(client, ch)
		errc <- err
	}()
	go func() {
		errc <- copyTerminalInput(ch, client)
	}()
	select {
	case <-client.Context().Done():
		return nil
	case err := <-errc:
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func terminalParams(q url.Values) map[string]string {
	params := map[string]string{}
	for _, key := range []string{"cols", "rows"} {
		if v := q.Get(key); v != "" {
			params[key] = v
		}
	}
	return params
}

type resizer interface {
	Resize(cols, rows int) error
}

func copyTerminalInput(ch plugin.Channel, client plugin.ClientStream) error {
	buf := make([]byte, 32<<10)
	for {
		n, err := client.Read(buf)
		if n > 0 {
			frame := buf[:n]
			if len(frame) > 1 && frame[0] == 0 {
				_ = handleTerminalControl(ch, frame[1:])
			} else if _, werr := ch.Write(frame); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}

func handleTerminalControl(ch plugin.Channel, frame []byte) error {
	var msg struct {
		Type string `json:"type"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if err := json.Unmarshal(frame, &msg); err != nil || msg.Type != "resize" {
		return err
	}
	if r, ok := ch.(resizer); ok {
		return r.Resize(msg.Cols, msg.Rows)
	}
	return nil
}

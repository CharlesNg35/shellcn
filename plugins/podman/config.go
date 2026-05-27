package podman

import "github.com/charlesng/shellcn/internal/plugin"

// configSchema is the Podman connection config. Podman keeps its own schema: the
// socket defaults to the rootful path, and the help text points rootless users
// at their runtime socket. Shown only for direct transport.
func configSchema() plugin.Schema {
	directOnly := plugin.Condition{AllOf: []plugin.Rule{{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)}}}
	directUnix := plugin.Condition{AllOf: []plugin.Rule{
		{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)},
		{Field: "endpoint_type", Op: plugin.OpEq, Value: "unix"},
	}}
	directTCP := plugin.Condition{AllOf: []plugin.Rule{
		{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)},
		{Field: "endpoint_type", Op: plugin.OpEq, Value: "tcp"},
	}}
	return plugin.Schema{Groups: []plugin.Group{{
		Name: "Endpoint",
		Fields: []plugin.Field{
			{Key: "endpoint_type", Label: "Endpoint", Type: plugin.FieldSelect, Required: true, Default: "unix", VisibleWhen: &directOnly, Options: []plugin.Option{
				{Label: "Unix socket", Value: "unix"},
				{Label: "TCP host", Value: "tcp"},
			}},
			{Key: "socket_path", Label: "Socket path", Type: plugin.FieldText, Required: true, Default: defaultSocket, Help: "Rootless: $XDG_RUNTIME_DIR/podman/podman.sock. Start with `podman system service`.", VisibleWhen: &directUnix},
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "podman.example.internal", VisibleWhen: &directTCP},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Required: true, Default: 8080, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}, VisibleWhen: &directTCP},
		},
	}}}
}

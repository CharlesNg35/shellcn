package swarm

import "github.com/charlesng/shellcn/internal/plugin"

// configSchema is the Swarm connection config: the Docker daemon endpoint on a
// manager node, shown only for direct transport (agent transport tunnels to the
// socket). Swarm keeps its own schema rather than sharing docker's.
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
		Name: "Manager",
		Fields: []plugin.Field{
			{Key: "endpoint_type", Label: "Endpoint", Type: plugin.FieldSelect, Required: true, Default: "unix", VisibleWhen: &directOnly, Options: []plugin.Option{
				{Label: "Unix socket", Value: "unix"},
				{Label: "TCP host", Value: "tcp"},
			}},
			{Key: "socket_path", Label: "Socket path", Type: plugin.FieldText, Required: true, Default: defaultSocket, VisibleWhen: &directUnix},
			{Key: "host", Label: "Manager host", Type: plugin.FieldText, Required: true, Placeholder: "manager.swarm.internal", VisibleWhen: &directTCP},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Required: true, Default: 2375, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}, VisibleWhen: &directTCP},
		},
	}}}
}

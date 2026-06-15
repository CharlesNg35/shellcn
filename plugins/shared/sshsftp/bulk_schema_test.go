package sshsftp

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestBulkRoutesDeclareActionableInputSchemas(t *testing.T) {
	routes := map[string]plugin.Route{}
	for _, route := range Routes("test", "test", false) {
		routes[route.ID] = route
	}
	for _, id := range []string{"test.sftp.chmod", "test.sftp.archive"} {
		if routes[id].Input == nil {
			t.Fatalf("%s missing input schema", id)
		}
	}
	if routes["test.sftp.transfer"].Method != plugin.MethodWS || routes["test.sftp.transfer"].Stream == nil {
		t.Fatalf("transfer route should be websocket-backed: %+v", routes["test.sftp.transfer"])
	}

	chmodMode := requireBulkField(t, routes["test.sftp.chmod"].Input, "mode")
	if chmodMode.Type != plugin.FieldAutocomplete || len(chmodMode.Options) < 2 || len(chmodMode.Validators) == 0 {
		t.Fatalf("chmod mode should suggest common octal modes and validate input: %+v", chmodMode)
	}
	archivePaths := requireBulkField(t, routes["test.sftp.archive"].Input, "paths")
	if archivePaths.Type != plugin.FieldArray || archivePaths.MinItems != 1 || archivePaths.Item == nil {
		t.Fatalf("archive paths should be a non-empty path array: %+v", archivePaths)
	}
}

func requireBulkField(t *testing.T, schema *plugin.Schema, key string) plugin.Field {
	t.Helper()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("missing field %q in schema %+v", key, schema)
	return plugin.Field{}
}

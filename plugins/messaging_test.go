package plugins

import (
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
)

func TestMessagingPluginsValidateAndRegister(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"rabbitmq", "kafka", "nats"} {
		proj, ok := reg.Projection(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		if proj.Category.Key != plugin.CategoryMessaging {
			t.Fatalf("%s category: got %q want %q", name, proj.Category.Key, plugin.CategoryMessaging)
		}
		if proj.Layout != plugin.LayoutSidebarTree {
			t.Fatalf("%s should use sidebar tree layout, got %q", name, proj.Layout)
		}
		if len(proj.Resources) == 0 || len(proj.Actions) == 0 {
			t.Fatalf("%s should expose resources and actions", name)
		}
		if len(proj.SupportedTransports) != 1 || proj.SupportedTransports[0] != plugin.TransportDirect {
			t.Fatalf("%s should be direct transport only: %+v", name, proj.SupportedTransports)
		}
	}
}

func TestMessagingCredentialCompatibility(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"rabbitmq", "kafka", "nats"} {
		if !reg.CredentialKindSupportsProtocol(plugin.CredentialBasicAuth, name) {
			t.Fatalf("basic auth credential should support %s", name)
		}
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialBearerToken, "nats") {
		t.Fatal("bearer token credential should support nats")
	}
	for _, name := range []string{"rabbitmq", "kafka"} {
		if reg.CredentialKindSupportsProtocol(plugin.CredentialBearerToken, name) {
			t.Fatalf("%s should not advertise bearer token credentials", name)
		}
	}
}

func TestMessagingAuthSchemasAreProtocolSpecific(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	rabbit, _ := reg.Manifest("rabbitmq")
	if !fieldMap(rabbit.Config)["management_url"] {
		t.Fatal("rabbitmq should expose management_url")
	}
	if fieldMap(rabbit.Config)["brokers"] || fieldMap(rabbit.Config)["urls"] {
		t.Fatal("rabbitmq should not expose kafka/nats endpoint fields")
	}

	kafka, _ := reg.Manifest("kafka")
	if !fieldMap(kafka.Config)["brokers"] {
		t.Fatal("kafka should expose brokers")
	}
	if fieldMap(kafka.Config)["management_url"] || fieldMap(kafka.Config)["urls"] {
		t.Fatal("kafka should not expose rabbitmq/nats endpoint fields")
	}

	nats, _ := reg.Manifest("nats")
	if !fieldMap(nats.Config)["urls"] || !fieldMap(nats.Config)["token"] {
		t.Fatal("nats should expose urls and token auth")
	}
	if fieldMap(nats.Config)["management_url"] || fieldMap(nats.Config)["brokers"] {
		t.Fatal("nats should not expose rabbitmq/kafka endpoint fields")
	}
}

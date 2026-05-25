package noop_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/noop"
)

func routeByID(p *noop.Plugin, id string) plugin.Route {
	for _, r := range p.Routes() {
		if r.ID == id {
			return r
		}
	}
	return plugin.Route{}
}

func TestManifestValidates(t *testing.T) {
	p := noop.New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("noop manifest invalid: %v", err)
	}
}

func TestListReturnsData(t *testing.T) {
	p := noop.New()
	rc := plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, sessionStub{}, nil, nil, nil)
	out, err := routeByID(p, "noop.list").Handle(rc)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if out == nil {
		t.Fatal("list returned nil")
	}
}

func TestEchoStream(t *testing.T) {
	p := noop.New()
	client, server := net.Pipe()
	rc := plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, sessionStub{}, nil, nil, nil)

	done := make(chan error, 1)
	go func() { done <- routeByID(p, "noop.echo").Stream(rc, &pipeStream{Conn: server}) }()

	_ = client.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	if _, err := client.Read(buf); err != nil { // greeting
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Errorf("echo mismatch: got %q", buf[:n])
	}

	_ = client.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("stream did not return after client close")
	}
}

type pipeStream struct{ net.Conn }

func (p *pipeStream) Context() context.Context { return context.Background() }

type sessionStub struct{}

func (sessionStub) HealthCheck(context.Context) error { return nil }
func (sessionStub) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}
func (sessionStub) Close() error { return nil }

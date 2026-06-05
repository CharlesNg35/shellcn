package loopback

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestCloseClosesActiveConnections(t *testing.T) {
	upstreams := make(chan net.Conn, 1)
	bridge, err := New(func(context.Context) (net.Conn, error) {
		local, upstream := net.Pipe()
		upstreams <- upstream
		return local, nil
	})
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}

	client, err := net.Dial("tcp", bridge.Addr())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer func() { _ = client.Close() }()

	var upstream net.Conn
	select {
	case upstream = <-upstreams:
		defer func() { _ = upstream.Close() }()
	case <-time.After(time.Second):
		t.Fatal("bridge did not dial upstream")
	}

	if err := bridge.Close(); err != nil {
		t.Fatalf("close bridge: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Fatal("client read should fail after bridge close")
	}

	_ = upstream.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := upstream.Read(make([]byte, 1)); err == nil {
		t.Fatal("upstream read should fail after bridge close")
	}
}

func TestCloseCancelsPendingDial(t *testing.T) {
	dialStarted := make(chan struct{})
	dialDone := make(chan error, 1)
	bridge, err := New(func(ctx context.Context) (net.Conn, error) {
		close(dialStarted)
		<-ctx.Done()
		dialDone <- ctx.Err()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}

	client, err := net.Dial("tcp", bridge.Addr())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case <-dialStarted:
	case <-time.After(time.Second):
		t.Fatal("bridge did not start dial")
	}

	if err := bridge.Close(); err != nil {
		t.Fatalf("close bridge: %v", err)
	}

	select {
	case err := <-dialDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("dial err = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending dial was not cancelled")
	}

	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	_, _ = client.Read(make([]byte, 1))
}

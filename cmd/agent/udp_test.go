package main

import (
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/charlesng35/shellcn/internal/transport"
)

func TestProxyStreamUDPEcho(t *testing.T) {
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc.Close() }()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := pc.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteToUDP(buf[:n], addr)
		}
	}()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	go proxyStream(slog.New(slog.NewTextHandler(io.Discard, nil)), server, transport.AgentProxyTarget{
		Mode:    transport.AgentModeUDP,
		Address: pc.LocalAddr().String(),
	})

	conn := transport.NewDatagramConn(client)
	for _, msg := range []string{"ping", "snmp"} {
		if _, err := conn.Write([]byte(msg)); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 2048)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if string(buf[:n]) != msg {
			t.Fatalf("echo = %q; want %q", buf[:n], msg)
		}
	}
}

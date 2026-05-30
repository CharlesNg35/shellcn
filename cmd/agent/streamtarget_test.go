package main

import (
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/charlesng35/shellcn/internal/transport"
)

func TestStreamTargetForwardReadsPreamble(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	go func() {
		_ = transport.WriteStreamTarget(client, "tcp", "172.18.0.5:80")
		_ = client.Close()
	}()
	network, addr := streamTarget(slog.New(slog.NewTextHandler(io.Discard, nil)), server,
		transport.AgentProxyTarget{Mode: transport.AgentModeUnix, Address: "/sock", Forward: true})
	if network != "tcp" || addr != "172.18.0.5:80" {
		t.Errorf("forward target = %q,%q; want tcp,172.18.0.5:80", network, addr)
	}
}

func TestStreamTargetLegacyUsesDeclaredAddress(t *testing.T) {
	_, server := net.Pipe()
	network, addr := streamTarget(slog.New(slog.NewTextHandler(io.Discard, nil)), server,
		transport.AgentProxyTarget{Mode: transport.AgentModeUnix, Address: "/var/run/docker.sock"})
	if network != "unix" || addr != "/var/run/docker.sock" {
		t.Errorf("legacy target = %q,%q; want unix,/var/run/docker.sock", network, addr)
	}
}

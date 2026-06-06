package dockerengine

import (
	"net/netip"
	"reflect"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestSplitProxyPath(t *testing.T) {
	cases := []struct {
		in                   string
		kind, id, port, rest string
		ok                   bool
	}{
		{"/container/abc/80/foo/bar", "container", "abc", "80", "foo/bar", true},
		{"/service/web/8080/", "service", "web", "8080", "", true},
		{"/container/abc/https:8443/", "container", "abc", "https:8443", "", true},
		{"/container/abc/80", "container", "abc", "80", "", true},
		{"/pods/x/y/z", "", "", "", "", false},
		{"/container/abc", "", "", "", "", false},
	}
	for _, c := range cases {
		kind, id, port, rest, ok := splitProxyPath(c.in)
		if kind != c.kind || id != c.id || port != c.port || rest != c.rest || ok != c.ok {
			t.Errorf("splitProxyPath(%q) = %q,%q,%q,%q,%v; want %q,%q,%q,%q,%v", c.in, kind, id, port, rest, ok, c.kind, c.id, c.port, c.rest, c.ok)
		}
	}
}

func TestPickWebPort(t *testing.T) {
	cases := []struct {
		name  string
		ports []int
		want  string
		isErr bool
	}{
		{"picks the lowest reachable port", []int{9000, 80}, "80", false},
		{"lowest even when several", []int{9000, 7000}, "7000", false},
		{"marks TLS ports", []int{443}, "https:443", false},
		{"detects non-standard TLS ports", []int{9443}, "https:9443", false},
		{"no ports", nil, "", true},
	}
	for _, c := range cases {
		got, err := pickWebPort(c.ports)
		if c.isErr {
			if err == nil {
				t.Errorf("%s: expected error", c.name)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: got %q,%v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestContainerPortOptions(t *testing.T) {
	got := containerPortOptions([]int{8443, 80, 80})
	want := []plugin.Option{
		{Label: "HTTP 80/tcp", Value: "80"},
		{Label: "HTTPS 8443/tcp", Value: "https:8443"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("containerPortOptions = %#v, want %#v", got, want)
	}
}

func TestContainerIP(t *testing.T) {
	if _, ok := containerIP(nil); ok {
		t.Error("nil network settings should yield no IP")
	}
	ns := &container.NetworkSettings{Networks: map[string]*network.EndpointSettings{
		"bridge": {IPAddress: netip.MustParseAddr("172.17.0.2")},
	}}
	if ip, ok := containerIP(ns); !ok || ip != "172.17.0.2" {
		t.Errorf("containerIP = %q,%v; want 172.17.0.2,true", ip, ok)
	}
	blank := &container.NetworkSettings{Networks: map[string]*network.EndpointSettings{"x": {}}}
	if _, ok := containerIP(blank); ok {
		t.Error("a network without a valid IP should yield no IP")
	}
}

// publishedNS builds network settings with one bridge IP and a published port.
func publishedNS(ip string, internal, hostPort string) *container.NetworkSettings {
	return &container.NetworkSettings{
		Networks: map[string]*network.EndpointSettings{"bridge": {IPAddress: netip.MustParseAddr(ip)}},
		Ports:    network.PortMap{network.MustParsePort(internal + "/tcp"): {{HostPort: hostPort}}},
	}
}

func TestProxyDialTarget(t *testing.T) {
	ns := publishedNS("172.18.0.5", "80", "8080")

	// Local daemon: reach the container's own IP and internal port.
	local := &Session{endpoint: endpoint{network: "unix", address: "/var/run/docker.sock"}}
	host, port, ok := local.proxyDialTarget(ns, "80")
	if !ok || host != "172.18.0.5" || port != "80" {
		t.Errorf("unix dial target = %q:%q,%v; want 172.18.0.5:80", host, port, ok)
	}

	// Remote daemon: reach the daemon host on the published port.
	remote := &Session{endpoint: endpoint{network: "tcp", address: "dockerhost:2375"}}
	host, port, ok = remote.proxyDialTarget(ns, "80")
	if !ok || host != "dockerhost" || port != "8080" {
		t.Errorf("tcp dial target = %q:%q,%v; want dockerhost:8080", host, port, ok)
	}

	// Remote daemon, port not published: not reachable.
	if _, _, ok := remote.proxyDialTarget(publishedNS("172.18.0.5", "9999", "9999"), "80"); ok {
		t.Error("an unpublished port should be unreachable over a remote daemon")
	}
}

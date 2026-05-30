package swarm

import (
	"testing"

	"github.com/moby/moby/api/types/swarm"
)

func TestPickServicePort(t *testing.T) {
	cases := []struct {
		name  string
		ports []swarm.PortConfig
		want  string
		isErr bool
	}{
		{
			"unnamed ports pick the lowest published",
			[]swarm.PortConfig{
				{PublishedPort: 30002, TargetPort: 80, Protocol: "tcp"},
				{PublishedPort: 30001, TargetPort: 9000, Protocol: "tcp"},
			},
			"30001", false,
		},
		{
			"marks TLS target ports",
			[]swarm.PortConfig{{PublishedPort: 9443, TargetPort: 443, Protocol: "tcp"}},
			"https:9443", false,
		},
		{
			"detects non-standard TLS target ports",
			[]swarm.PortConfig{{PublishedPort: 30001, TargetPort: 9443, Protocol: "tcp"}},
			"https:30001", false,
		},
		{
			"falls back to the lowest published",
			[]swarm.PortConfig{
				{PublishedPort: 8081, TargetPort: 7000, Protocol: "tcp"},
				{PublishedPort: 8080, TargetPort: 6000, Protocol: "tcp"},
			},
			"8080", false,
		},
		{
			"ignores unpublished ports",
			[]swarm.PortConfig{{PublishedPort: 0, TargetPort: 80, Protocol: "tcp"}},
			"", true,
		},
		{
			"ignores non-TCP",
			[]swarm.PortConfig{{PublishedPort: 5300, TargetPort: 53, Protocol: "udp"}},
			"", true,
		},
		{
			"scheme + preference follow the port name",
			[]swarm.PortConfig{
				{PublishedPort: 30001, TargetPort: 9000, Protocol: "tcp"},
				{PublishedPort: 30002, TargetPort: 9443, Protocol: "tcp", Name: "https"},
			},
			"https:30002", false,
		},
		{
			"prefers ingress over host-mode",
			[]swarm.PortConfig{
				{PublishedPort: 30080, TargetPort: 80, Protocol: "tcp", PublishMode: swarm.PortConfigPublishModeHost},
				{PublishedPort: 30090, TargetPort: 9000, Protocol: "tcp", PublishMode: swarm.PortConfigPublishModeIngress},
			},
			"30090", false,
		},
		{
			"falls back to host-mode when no ingress",
			[]swarm.PortConfig{{PublishedPort: 30080, TargetPort: 80, Protocol: "tcp", PublishMode: swarm.PortConfigPublishModeHost}},
			"30080", false,
		},
	}
	for _, c := range cases {
		got, err := pickServicePort(c.ports)
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

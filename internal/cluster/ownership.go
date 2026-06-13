// Package cluster defines platform-neutral ownership primitives for live
// gateway state that must remain in process memory.
package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	ErrOwnedElsewhere = errors.New("cluster: owner is another instance")
	ErrLeaseExpired   = errors.New("cluster: lease expired")
)

type ClaimMode string

const (
	ClaimExclusive ClaimMode = "exclusive"
	ClaimReplace   ClaimMode = "replace"
)

type InstanceRef struct {
	ID          string
	InternalURL string
	StartedAt   time.Time
}

func NewInstanceRef(id, internalURL string) InstanceRef {
	id = strings.TrimSpace(id)
	if id == "" {
		id = defaultInstanceID()
	}
	return InstanceRef{ID: id, InternalURL: strings.TrimSpace(internalURL), StartedAt: time.Now().UTC()}
}

func defaultInstanceID() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "shellcn"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

type OwnerRef struct {
	Instance  InstanceRef
	Key       string
	LeaseID   string
	ExpiresAt time.Time
}

func (o OwnerRef) IsLocal(instance InstanceRef) bool {
	return o.Instance.ID != "" && o.Instance.ID == instance.ID
}

type ClaimOptions struct {
	Mode ClaimMode
	TTL  time.Duration
}

func (o ClaimOptions) withDefaults() ClaimOptions {
	if o.Mode == "" {
		o.Mode = ClaimExclusive
	}
	if o.TTL <= 0 {
		o.TTL = 30 * time.Second
	}
	return o
}

type Lease interface {
	Owner() OwnerRef
	Renew(ctx context.Context) error
	Release(ctx context.Context) error
}

type OwnerRegistry interface {
	Claim(ctx context.Context, key string, instance InstanceRef, opts ClaimOptions) (Lease, error)
	Get(ctx context.Context, key string) (OwnerRef, bool, error)
}

func AgentOwnerKey(connectionID string) string {
	return "agent:" + connectionID
}

func SessionOwnerKey(connectionID, ownerScope string) string {
	return "session:" + connectionID + ":" + ownerScope
}

func DiscoverInternalURL(port string, tlsEnabled bool) string {
	port = normalizePort(port)
	if port == "" {
		return ""
	}
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	host := discoverInstanceHost()
	if host == "" {
		return ""
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

func PortFromListenAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	_, port, err := net.SplitHostPort(addr)
	if err == nil && port != "" {
		return port
	}
	if strings.HasPrefix(addr, ":") {
		return strings.TrimPrefix(addr, ":")
	}
	if i := strings.LastIndexByte(addr, ':'); i >= 0 && i < len(addr)-1 {
		return addr[i+1:]
	}
	return ""
}

func normalizePort(port string) string {
	port = strings.TrimSpace(port)
	port = strings.TrimPrefix(port, ":")
	return port
}

func discoverInstanceHost() string {
	for _, key := range []string{
		"SHELLCN_INSTANCE_IP",
		"POD_IP",
		"KUBERNETES_POD_IP",
		"MY_POD_IP",
		"CONTAINER_IP",
		"HOST_IP",
	} {
		if host := cleanDiscoveredHost(os.Getenv(key)); host != "" {
			return host
		}
	}
	if host := discoverECSHost(); host != "" {
		return host
	}
	if host := discoverInterfaceHost(); host != "" {
		return host
	}
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return ""
	}
	addrs, err := net.LookupHost(strings.TrimSpace(host))
	if err == nil {
		for _, addr := range addrs {
			if host := cleanDiscoveredHost(addr); host != "" {
				return host
			}
		}
	}
	return cleanDiscoveredHost(host)
}

func discoverECSHost() string {
	metadataURL := strings.TrimSpace(os.Getenv("ECS_CONTAINER_METADATA_URI_V4"))
	if metadataURL == "" {
		return ""
	}
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(metadataURL)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	var meta struct {
		Networks []struct {
			IPv4Addresses []string `json:"IPv4Addresses"`
			IPv6Addresses []string `json:"IPv6Addresses"`
		} `json:"Networks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return ""
	}
	for _, network := range meta.Networks {
		for _, addr := range network.IPv4Addresses {
			if host := cleanDiscoveredHost(addr); host != "" {
				return host
			}
		}
		for _, addr := range network.IPv6Addresses {
			if host := cleanDiscoveredHost(addr); host != "" {
				return host
			}
		}
	}
	return ""
}

func discoverInterfaceHost() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var fallback string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			host, ok := hostFromAddr(addr)
			if !ok {
				continue
			}
			ip := net.ParseIP(host)
			if ip == nil || !isUsableInstanceIP(ip) {
				continue
			}
			if ip.To4() != nil && isPrivateIPv4(ip) {
				return host
			}
			if fallback == "" {
				fallback = host
			}
		}
	}
	return fallback
}

func hostFromAddr(addr net.Addr) (string, bool) {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP.String(), true
	case *net.IPAddr:
		return v.IP.String(), true
	default:
		return "", false
	}
}

func cleanDiscoveredHost(host string) string {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" || strings.EqualFold(host, "localhost") {
		return ""
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host
	}
	if !isUsableInstanceIP(ip) {
		return ""
	}
	return ip.String()
}

func isUsableInstanceIP(ip net.IP) bool {
	return ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}

func isPrivateIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 10 ||
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
		(ip4[0] == 192 && ip4[1] == 168)
}

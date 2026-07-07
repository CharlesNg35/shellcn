package server

import (
	"net/http"
	"testing"
)

func TestClientIPUsesForwardedHeaderFromTrustedProxy(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = "10.0.1.8:42512"
	req.Header.Set("Forwarded", `for="203.0.113.24";proto=https`)
	req.Header.Set("X-Forwarded-For", "198.51.100.10")

	if got := clientIP(req); got != "203.0.113.24" {
		t.Fatalf("clientIP = %q, want forwarded client", got)
	}
}

func TestClientIPUsesXForwardedForFromTrustedProxy(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = "127.0.0.1:42512"
	req.Header.Set("X-Forwarded-For", "203.0.113.24, 10.0.1.8")

	if got := clientIP(req); got != "203.0.113.24" {
		t.Fatalf("clientIP = %q, want first forwarded client", got)
	}
}

func TestClientIPIgnoresForwardedHeaderFromPublicPeer(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.RemoteAddr = "198.51.100.10:42512"
	req.Header.Set("X-Forwarded-For", "203.0.113.24")

	if got := clientIP(req); got != "198.51.100.10" {
		t.Fatalf("clientIP = %q, want direct peer", got)
	}
}

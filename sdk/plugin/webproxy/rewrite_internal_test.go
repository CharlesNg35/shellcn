package webproxy

import "testing"

const (
	pfx    = "/proxy/x"
	pub    = "app.example.com"
	upHost = "10.1.2.3:8080"
)

func TestMapLocation(t *testing.T) {
	cases := []struct {
		name, loc, src, want string
	}{
		{"root relative", "/login", "", "/proxy/x/login"},
		{"bare slash", "/", "", "/proxy/x/"},
		{"nested root relative", "/a/b/c?d=1#e", "", "/proxy/x/a/b/c?d=1#e"},
		{"already prefixed root", "/proxy/x/login", "", "/proxy/x/login"},
		{"exact prefix", "/proxy/x", "", "/proxy/x"},

		{"public host absolute", "https://app.example.com/login", "", "/proxy/x/login"},
		{"public host other scheme", "http://app.example.com/login", "", "/proxy/x/login"},
		{"public host root", "https://app.example.com", "", "/proxy/x/"},
		{"public host with query", "https://app.example.com/o?a=1&b=2", "", "/proxy/x/o?a=1&b=2"},
		{"public host already prefixed", "https://app.example.com/proxy/x/login", "", "/proxy/x/login"},

		{"upstream host absolute", "http://10.1.2.3:8080/dash", "", "/proxy/x/dash"},
		{"protocol relative public", "//app.example.com/p", "", "/proxy/x/p"},
		{"protocol relative upstream", "//10.1.2.3:8080/p", "", "/proxy/x/p"},

		{"external host left alone", "https://accounts.google.com/o/oauth2/auth?x=1", "", "https://accounts.google.com/o/oauth2/auth?x=1"},
		{"protocol relative external", "//cdn.other.net/lib.js", "", "//cdn.other.net/lib.js"},
		{"path relative left alone", "next", "", "next"},
		{"dot relative left alone", "./next", "", "./next"},
		{"empty left alone", "", "", ""},
		{"case-insensitive host", "https://APP.example.com/x", "", "/proxy/x/x"},

		{"source prefix mapped", "/src/page", "/src", "/proxy/x/page"},
		{"source prefix beats parse", "/src", "/src", "/proxy/x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mapLocation(c.loc, pfx, pub, upHost, c.src); got != c.want {
				t.Fatalf("mapLocation(%q) = %q, want %q", c.loc, got, c.want)
			}
		})
	}
}

func TestStripOrigins(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"absolute https", `a="https://app.example.com/x"`, `a="/proxy/x/x"`},
		{"absolute http", `a="http://app.example.com/x"`, `a="/proxy/x/x"`},
		{"already prefixed not doubled", `a="https://app.example.com/proxy/x/x"`, `a="/proxy/x/x"`},
		{"multiple occurrences", `https://app.example.com/a https://app.example.com/b`, `/proxy/x/a /proxy/x/b`},
		{"bare origin", `go to https://app.example.com"`, `go to /proxy/x"`},
		{"unrelated origin untouched", `https://other.net/x`, `https://other.net/x`},
		{"no origin", `/already/relative`, `/already/relative`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripOrigins(c.in, pfx, "https://"+pub, "http://"+pub)
			if got != c.want {
				t.Fatalf("stripOrigins(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRewriteCookiePath(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"path scoped", "sid=abc; Path=/; HttpOnly", "sid=abc; Path=/proxy/x; HttpOnly"},
		{"path added when absent", "sid=abc; HttpOnly", "sid=abc; HttpOnly; Path=/proxy/x"},
		{"domain dropped", "sid=abc; Domain=10.1.2.3; Path=/", "sid=abc; Path=/proxy/x"},
		{"domain dropped case", "sid=abc; domain=pod.local", "sid=abc; Path=/proxy/x"},
		{"host cookie untouched", "__Host-sec=z; Path=/; Secure", "__Host-sec=z; Path=/; Secure"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rewriteCookiePath(c.in, pfx); got != c.want {
				t.Fatalf("rewriteCookiePath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

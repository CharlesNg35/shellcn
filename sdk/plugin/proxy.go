package plugin

import "net/http"

// ProxyPrefixHeader carries the connection's public proxy mount on requests the
// core hands to a Session's ServeHTTPProxy. The core stamps it after stripping
// the mount from the path, so a proxy that rewrites HTML/redirects/cookies can
// re-prefix without knowing the gateway's URL layout.
const ProxyPrefixHeader = "X-Shellcn-Proxy-Prefix"

// RequestProxyPrefix returns the public proxy mount for a ServeHTTPProxy
// request, without a trailing slash (empty when absent).
func RequestProxyPrefix(r *http.Request) string {
	return r.Header.Get(ProxyPrefixHeader)
}

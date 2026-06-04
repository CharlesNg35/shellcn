package plugin

import "net/http"

// ProxyPrefixHeader carries the connection's public proxy mount on requests
// the core hands to ServeHTTPProxy.
const ProxyPrefixHeader = "X-Shellcn-Proxy-Prefix"

// RequestProxyPrefix returns the request's proxy mount (no trailing slash) and
// strips the header so it is never forwarded upstream.
func RequestProxyPrefix(r *http.Request) string {
	prefix := r.Header.Get(ProxyPrefixHeader)
	r.Header.Del(ProxyPrefixHeader)
	return prefix
}

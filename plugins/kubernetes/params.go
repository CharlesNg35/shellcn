package kubernetes

import (
	"strconv"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// param reads a renderer-supplied value from the path params, falling back to
// the query string (WS routes carry stream options as query values).
func param(rc *plugin.RequestContext, key string) string {
	if v := rc.Param(key); v != "" {
		return v
	}
	return rc.Query().Get(key)
}

func boolParam(rc *plugin.RequestContext, key string, def bool) bool {
	switch strings.ToLower(param(rc, key)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}

func intParam(rc *plugin.RequestContext, key string) int {
	n, _ := strconv.Atoi(param(rc, key))
	return n
}

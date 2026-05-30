package webproxy_test

import (
	"testing"

	"github.com/charlesng35/shellcn/plugins/shared/webproxy"
)

func TestIsTLSPort(t *testing.T) {
	tls := []int{443, 8443, 9443, 10443, 4443}
	for _, p := range tls {
		if !webproxy.IsTLSPort(p) {
			t.Errorf("IsTLSPort(%d) = false, want true", p)
		}
	}
	plain := []int{80, 8080, 3000, 8000, 5000, 22}
	for _, p := range plain {
		if webproxy.IsTLSPort(p) {
			t.Errorf("IsTLSPort(%d) = true, want false", p)
		}
	}
}

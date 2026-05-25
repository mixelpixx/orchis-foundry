package netguard

import (
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "::1", "10.0.0.5", "192.168.1.10", "172.16.4.4",
		"169.254.169.254", "0.0.0.0", "fe80::1", "fc00::1"}
	for _, s := range blocked {
		if !IsBlockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1::"}
	for _, s := range allowed {
		if IsBlockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s allowed", s)
		}
	}
}

func TestValidateOutboundURL(t *testing.T) {
	// Scheme + literal-IP checks don't need DNS.
	bad := []string{
		"ftp://example.com", "file:///etc/passwd", "javascript:alert(1)",
		"http://127.0.0.1:6379", "http://169.254.169.254/latest/meta-data/",
		"http://10.1.2.3/hook", "http://[::1]/x", "http://localhost/x", "not a url",
	}
	for _, u := range bad {
		if err := ValidateOutboundURL(u); err == nil {
			t.Errorf("expected %q rejected", u)
		}
	}
	// Public literal IPs should pass without touching DNS.
	good := []string{"https://8.8.8.8/hook", "http://1.1.1.1/path?x=1"}
	for _, u := range good {
		if err := ValidateOutboundURL(u); err != nil {
			t.Errorf("expected %q allowed, got %v", u, err)
		}
	}
}

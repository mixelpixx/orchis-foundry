// Package netguard validates outbound URLs (webhooks, BYO-model endpoints) to
// prevent SSRF — blocking requests to loopback, private, link-local, and
// cloud-metadata addresses. Used at configuration time and again before each
// outbound request (defense against DNS rebinding / stale targets).
package netguard

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// AllowInternal reports whether an operator has opted in to internal targets
// (e.g. an in-cluster service) via ORCHIS_ALLOW_INTERNAL_TARGETS=1. Off by default.
func AllowInternal() bool {
	v := os.Getenv("ORCHIS_ALLOW_INTERNAL_TARGETS")
	return v == "1" || strings.EqualFold(v, "true")
}

// ValidateOutboundURL returns an error if rawurl is not a safe public http(s)
// target. It rejects non-http(s) schemes and any host that resolves to a
// loopback / private / link-local (incl. 169.254.169.254 metadata) / unique-local
// address. Honors the AllowInternal opt-out.
func ValidateOutboundURL(rawurl string) error {
	u, err := url.Parse(strings.TrimSpace(rawurl))
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must be http(s)")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}
	if AllowInternal() {
		return nil
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("URL points to a blocked internal host")
	}

	var ips []net.IP
	if literal := net.ParseIP(host); literal != nil {
		ips = []net.IP{literal}
	} else {
		resolved, err := net.LookupIP(host)
		if err != nil || len(resolved) == 0 {
			return fmt.Errorf("could not resolve host")
		}
		ips = resolved
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return fmt.Errorf("URL points to a blocked internal/private address")
		}
	}
	return nil
}

// IsBlockedIP reports whether ip is in a range we refuse to call outbound.
func IsBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || // 127.0.0.0/8, ::1
		ip.IsPrivate() || // RFC1918 + fc00::/7 (Go 1.17+)
		ip.IsLinkLocalUnicast() || // 169.254.0.0/16 (incl. cloud metadata), fe80::/10
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() // 0.0.0.0, ::
}

// Package monitor implements the HTTP checking engine and SSRF protections.
package monitor

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidatePublicURL parses a user-supplied URL and rejects non-public destinations
// before any network connection is attempted. This is the first SSRF gate.
func ValidatePublicURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("url is required")
	}
	if len(raw) > 2048 {
		return nil, fmt.Errorf("url is too long")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("only http and https schemes are allowed")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url must include a host")
	}
	if u.User != nil {
		return nil, fmt.Errorf("url must not contain userinfo credentials")
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("url must include a hostname")
	}

	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") || lower == "metadata.google.internal" {
		return nil, fmt.Errorf("localhost and metadata hosts are not allowed")
	}

	// If the host is already a literal IP, validate it immediately.
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("destination IP is not a public address")
		}
	}

	return u, nil
}

// isPublicIP returns false for loopback, private, link-local, multicast, and
// other non-routable / special-use ranges (IPv4 and IPv6).
func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		ip.IsInterfaceLocalMulticast() {
		return false
	}

	// Extra IPv4 special-use ranges not covered by the helpers above.
	if v4 := ip.To4(); v4 != nil {
		// 0.0.0.0/8
		if v4[0] == 0 {
			return false
		}
		// 100.64.0.0/10 — Carrier-grade NAT (RFC 6598)
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return false
		}
		// 169.254.0.0/16 — link-local (also IsLinkLocalUnicast, kept for clarity)
		if v4[0] == 169 && v4[1] == 254 {
			return false
		}
		// 192.0.0.0/24 — IETF protocol assignments
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 0 {
			return false
		}
		// 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24 — documentation
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 2 {
			return false
		}
		if v4[0] == 198 && v4[1] == 51 && v4[2] == 100 {
			return false
		}
		if v4[0] == 203 && v4[1] == 0 && v4[2] == 113 {
			return false
		}
		// 198.18.0.0/15 — benchmarking
		if v4[0] == 198 && (v4[1] == 18 || v4[1] == 19) {
			return false
		}
		// 224.0.0.0/4 multicast already covered; 240.0.0.0/4 reserved
		if v4[0] >= 240 {
			return false
		}
	}

	// IPv6 unique local (fc00::/7) is covered by IsPrivate in modern Go.
	// IPv4-mapped IPv6 addresses: re-check the embedded IPv4.
	if ip.To4() == nil && ip.To16() != nil {
		if mapped := ip.To4(); mapped != nil {
			return isPublicIP(mapped)
		}
	}

	return true
}

// filterPublicIPs keeps only globally routable addresses from a DNS lookup.
func filterPublicIPs(ips []net.IP) ([]net.IP, error) {
	var public []net.IP
	for _, ip := range ips {
		if isPublicIP(ip) {
			public = append(public, ip)
		}
	}
	if len(public) == 0 {
		return nil, fmt.Errorf("host resolves only to non-public addresses")
	}
	return public, nil
}

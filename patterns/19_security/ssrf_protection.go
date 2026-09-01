package security

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	ErrSSRFBlocked      = errors.New("security violation: target URL points to a private/internal IP address")
	ErrDisallowedScheme = errors.New("security violation: only HTTP and HTTPS schemes are permitted")
)

// IsPrivateOrReservedIP checks if an IP belongs to private, loopback, or cloud-metadata ranges.
func IsPrivateOrReservedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	// Loopback (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return true
	}

	// Private (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
	if ip.IsPrivate() {
		return true
	}

	// Unspecified (0.0.0.0) or Multicast
	if ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}

	// AWS/GCP/Azure link-local metadata (169.254.169.254)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Direct check for 169.254.169.254
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}

	return false
}

// ValidateTargetURL prevents Server-Side Request Forgery by ensuring the destination is safe for outbound requests.
func ValidateTargetURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// 1. Enforce safe scheme
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: '%s'", ErrDisallowedScheme, scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}
	hostname := u.Hostname()

	// 2. Direct IP check
	if ip := net.ParseIP(hostname); ip != nil {
		if IsPrivateOrReservedIP(ip) {
			return fmt.Errorf("%w: %s", ErrSSRFBlocked, ip.String())
		}
		return nil
	}

	// 3. Reject known localhost names
	if strings.EqualFold(hostname, "localhost") {
		return fmt.Errorf("%w: localhost", ErrSSRFBlocked)
	}

	return nil
}

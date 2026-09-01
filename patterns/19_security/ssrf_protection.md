# Server-Side Request Forgery (SSRF) Protection Pattern

## 1. Overview & Concept
The **Server-Side Request Forgery (SSRF) Protection Pattern** prevents backend servers from being coerced into making unauthorized outbound network requests to internal, private, loopback, or cloud-metadata endpoints. SSRF occurs when backend applications fetch remote resources (such as webhooks, avatar image URLs, OAuth callbacks, PDF generator engines, or dynamic RSS feeds) based on user-supplied URLs without strictly validating the destination IP address and protocol.

This pattern operates as a transport-level network gatekeeper:
1. **Strict Scheme Allowlisting:** Restricting URL protocols exclusively to `http` and `https` (blocking `file://`, `gopher://`, `ftp://`, `dict://`, `ldap://`).
2. **Private & Reserved IP Detection:** Inspecting destination IP addresses and actively blocking loopback (`127.0.0.0/8`), RFC-1918 private subnets (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`), link-local metadata addresses (`169.254.169.254`), multicast, and unspecified ranges (`0.0.0.0`).

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
SSRF vulnerabilities (CWE-918) allow attackers to leverage the trusted network perimeter of backend servers to bypass firewalls and access internal infrastructure.

### Failure Scenarios Without This Pattern
- **Cloud Metadata & IAM Credential Theft:** Attackers supply `http://169.254.169.254/latest/meta-data/iam/security-credentials/` to steal temporary AWS EC2/ECS/EKS IAM roles, leading to total cloud infrastructure takeover.
- **Internal Microservice & Admin API Exfiltration:** Bypassing ingress firewalls to call unauthenticated internal services, Kubernetes API servers (`https://kubernetes.default.svc`), or local Prometheus/management endpoints (`http://127.0.0.1:9090`).
- **Internal Database & Cache Manipulation:** Reaching Redis instances (`http://127.0.0.1:6379`) or Elasticsearch clusters (`http://internal-elastic:9200`) to flush data, write keys, or extract indexes.
- **Port Scanning & Network Topology Mapping:** Probing internal IP ranges and observing latency/error differences to map internal network layouts.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The validation flow evaluates the target URL structure and parses hostnames to ensure no internal IP or disallowed scheme can be queried:

```
        [ Untrusted Webhook URL: "http://169.254.169.254/latest" ]
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │       ValidateTargetURL       │
                   └───────────────┬───────────────┘
                                   │
       1. Scheme Check: Is scheme http or https?
          [file://, gopher://, etc. -> Reject ErrDisallowedScheme]
                                   │
       2. Hostname Check: Is hostname "localhost"?
          [Yes -> Reject ErrSSRFBlocked]
                                   │
       3. IP Address Resolution / Direct IP Check:
          IsPrivateOrReservedIP(ip)?
                                   │
       ┌───────────────────────────┴───────────────────────────┐
       ▼                                                       ▼
 [Private / Loopback / Cloud Metadata]                 [Public Public IP]
 [127.0.0.1, 10.0.0.0/8, 169.254.169.254]                      │
       │                                                       ▼
       ▼                                               [Validation Passes]
 [Reject ErrSSRFBlocked]                          [Proceed with HTTP Request]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Validate at DNS Resolution Time (DNS Rebinding Defense):** A critical vulnerability in naive SSRF validation is DNS rebinding (Time-of-Check to Time-of-Use / TOCTOU), where a domain resolves to a public IP during validation, but resolves to `127.0.0.1` when the HTTP client dials the connection. In production, configure a custom `net.Dialer.Control` or `net.Resolver` that validates the resolved IP immediately before the socket is created.
- **Disable HTTP Redirects to Unvetted Hosts:** A public URL might issue an HTTP 302 redirect to `http://169.254.169.254/`. Configure `http.Client.CheckRedirect` to validate redirect destination URLs before following.
- **Isolate Webhook Egress in a Dedicated DMZ:** Route outbound user-directed HTTP requests through an isolated proxy or distinct egress subnet with explicit network security group (NSG) rules denying access to private IP ranges.
- **Enforce Short Connection Timeouts:** Set strict dial, handshake, and response timeouts (e.g., 2-5 seconds) to prevent slowloris-style outbound socket exhaustion.

### Pitfalls to Avoid
- **Relying Only on String Filtering:** Filtering out `"127.0.0.1"` can be bypassed by decimal (`2130706433`), octal (`017700000001`), IPv6 mapped (`::ffff:127.0.0.1`), or alternative DNS records (`localtest.me`).
- **Ignoring IPv6 Ranges:** Ensure IPv6 loopback (`::1`), link-local (`fe80::/10`), and unique local (`fc00::/7`) addresses are fully checked.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/19_security/ssrf_protection.go`
```go
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
```

### Production Outbound Webhook Client Example
```go
func FetchRemoteWebhook(ctx context.Context, targetURL string) ([]byte, error) {
    // 1. Validate target URL before making network calls
    if err := security.ValidateTargetURL(targetURL); err != nil {
        return nil, fmt.Errorf("ssrf check failed: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
    if err != nil {
        return nil, err
    }

    client := &http.Client{
        Timeout: 5 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            // Re-validate redirect destination to prevent redirect SSRF
            return security.ValidateTargetURL(req.URL.String())
        },
    }

    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // Bound read to 1MB
}
```

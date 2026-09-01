# Multi-Strategy API Versioning

## Overview & Definition

As backend APIs evolve, breaking changes (renaming fields, modifying data types, restructuring endpoints) are inevitable. Managing multiple co-existing versions allows gradual migration without breaking legacy client integrations.

The **Multi-Strategy API Versioning** pattern (`ExtractAPIVersion`) provides a unified version resolver that inspects three standard versioning strategies in order of precedence:
1. **URI Path-Based Versioning (Most Common):** `/api/v1/users`, `/api/v2/users`.
2. **Custom Header-Based Versioning:** `X-API-Version: v2` or `X-API-Version: 2`.
3. **Content Negotiation / Vendor Media Type (`Accept` Header):** `Accept: application/vnd.company.v2+json`.
4. Falls back to a deterministic default version (`v1`) when no version indicator is specified.

---

## Problem Statement

Failing to implement a structured API versioning strategy leads to:

* **Breaking Production Clients:** Deploying breaking schema modifications to unversioned endpoints immediately breaks mobile apps that cannot be updated instantly by users.
* **Inconsistent Versioning Implementations:** Different engineering teams within the same company adopting divergent versioning approaches (some using paths, others using headers) complicates API Gateway routing.
* **Complex Route Proliferation:** Hardcoding ad-hoc version switches inside individual handlers creates messy conditional logic and tech debt.

---

## Architectural Mechanism & Flow

```
                      Incoming HTTP Request
                                |
                                v
               +----------------------------------+
               |      ExtractAPIVersion(r)        |
               +----------------------------------+
                                |
             +------------------+------------------+
             |                                     |
             v                                     v
   [Check URL Path Regex]                [Check X-API-Version]
   e.g. /api/(v1|v2)/...                 e.g. "v2" or "2"
             |                                     |
      [Match Found?]                        [Match Found?]
        /         \                           /         \
     [Yes]        [No]                     [Yes]        [No]
       |            |                        |            |
       v            +------------+-----------+            v
  Return Version                 |            [Check Accept Header Regex]
                                 |            e.g. application/vnd.*.v2+json
                                 |                        |
                                 |                 [Match Found?]
                                 |                   /         \
                                 |                [Yes]        [No]
                                 |                  |            |
                                 +------------------+            v
                                                            Return Default
                                                             Version (v1)
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Prefer URI Path Versioning for Public APIs:** Path versioning (`/api/v1/...`) is explicit, transparent in logs, easy to route in API Gateways (Envoy, Kong, AWS API Gateway), and developer-friendly.
* **Use Header Versioning for Microservice RPCs:** In internal service-to-service communication, header versioning (`X-API-Version` or `Accept`) allows version negotiation without modifying base routing trees.
* **Deprecate Old Versions with Sunsetting Headers:** Emit RFC 8594 `Sunset` and `Deprecation` headers on older versions to notify consumers of upcoming retirement dates.

### Common Pitfalls
* **Micro-Versioning (Over-Versioning):** Incrementing major API versions for non-breaking changes (such as adding an optional field) creates maintenance nightmares. Only bump versions for genuinely breaking modifications.
* **Branching Version Logic Inside Handlers:** Avoid `if version == "v2" { ... } else { ... }` inside database queries. Route to separate, dedicated version handlers at the router/controller layer.

---

## Code Walkthrough & Usage

### 1. Implementation (`api_versioning.go`)

```go
package apidesign

import (
	"net/http"
	"regexp"
	"strings"
)

// APIVersion represents supported API versions.
type APIVersion string

const (
	VersionV1 APIVersion = "v1"
	VersionV2 APIVersion = "v2"
)

var (
	pathVersionRegex  = regexp.MustCompile(`^/api/(v[1-9][0-9]*)/`)
	acceptHeaderRegex = regexp.MustCompile(`application/vnd\.[a-zA-Z0-9_-]+\.(v[1-9][0-9]*)\+json`)
)

// ExtractAPIVersion resolves target API version from URL path, X-API-Version header, or Accept header.
func ExtractAPIVersion(r *http.Request) APIVersion {
	// 1. Check URL path (e.g. /api/v1/orders)
	if matches := pathVersionRegex.FindStringSubmatch(r.URL.Path); len(matches) > 1 {
		return APIVersion(strings.ToLower(matches[1]))
	}

	// 2. Check X-API-Version header (e.g. X-API-Version: v2)
	if h := r.Header.Get("X-API-Version"); h != "" {
		h = strings.ToLower(strings.TrimSpace(h))
		if !strings.HasPrefix(h, "v") {
			h = "v" + h
		}
		return APIVersion(h)
	}

	// 3. Check Accept header (e.g. application/vnd.company.v2+json)
	if accept := r.Header.Get("Accept"); accept != "" {
		if matches := acceptHeaderRegex.FindStringSubmatch(accept); len(matches) > 1 {
			return APIVersion(strings.ToLower(matches[1]))
		}
	}

	// Default fallback version
	return VersionV1
}
```

### 2. Version Router Dispatch Example

```go
func VersionRoutingMiddleware(v1Handler, v2Handler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        version := ExtractAPIVersion(r)

        switch version {
        case VersionV2:
            v2Handler.ServeHTTP(w, r)
        case VersionV1:
            fallthrough
        default:
            v1Handler.ServeHTTP(w, r)
        }
    })
}
```

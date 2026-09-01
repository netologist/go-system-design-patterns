# Security Headers & CORS Middleware

## 1. Overview & Concept

Modern web APIs and SPAs (Single Page Applications) operate in hostile browser environments where cross-site scripting (XSS), clickjacking, MIME-sniffing, and cross-origin resource sharing (CORS) misconfigurations present constant security risks.

The **Security Headers & CORS Middleware** pattern establishes an essential defensive perimeter at the HTTP transport layer by:
1. **Enforcing OWASP Recommended Defensive Security Headers:** Automatically injecting headers (`Strict-Transport-Security`, `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`, `Referrer-Policy`) to harden browser clients against common client-side attacks.
2. **Standardizing Cross-Origin Resource Sharing (CORS):** Validating client `Origin` headers against a configurable whitelist, dynamically injecting `Access-Control-Allow-*` response headers, and short-circuiting HTTP `OPTIONS` preflight requests with `204 No Content`.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without centralized security headers and CORS enforcement:

* **Clickjacking & UI Redressing:** Missing `X-Frame-Options: DENY` allows attackers to embed your web application inside a transparent `<iframe>` on an external malicious site, tricking authenticated users into clicking buttons or submitting unauthorized forms.
* **MIME Sniffing Exploits:** Without `X-Content-Type-Options: nosniff`, browsers attempt to override declared MIME types and execute user-uploaded files (e.g. interpreting an uploaded `.png` containing JavaScript as `text/html`).
* **Man-In-The-Middle (MITM) Downgrade Attacks:** Without `Strict-Transport-Security (HSTS)`, initial HTTP requests from clients can be intercepted and downgraded before redirecting to HTTPS.
* **Overly Permissive CORS (`Access-Control-Allow-Origin: *` with Credentials):** Misconfigured CORS policies allow untrusted third-party origins to execute authenticated fetch requests and read sensitive JSON responses on behalf of logged-in corporate users.
* **Preflight Request Flooding:** Failing to handle `OPTIONS` requests efficiently causes excessive compute, database queries, and log noise from browser preflight handshakes.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The middleware executes in two distinct defensive layers:

```
                            Incoming Request
                                   │
                                   ▼
             ┌───────────────────────────────────────────┐
             │         SecurityHeadersMiddleware         │
             │ Injects: HSTS, Nosniff, Frame-Options,    │
             │          CSP, Referrer-Policy             │
             └─────────────────────┬─────────────────────┘
                                   │
                                   ▼
             ┌───────────────────────────────────────────┐
             │              CORSMiddleware               │
             │ Checks: r.Header.Get("Origin")            │
             └─────────────────────┬─────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │                             │
          [Origin Matches Whitelist]     [Origin Not Whitelisted]
                    │                             │
                    ▼                             ▼
        Set Access-Control-Allow-*         No CORS Headers Set
                    │                             │
          ┌─────────┴─────────┐                   │
          │                   │                   │
     [Method == OPTIONS]  [Method != OPTIONS]     │
          │                   │                   │
          ▼                   ▼                   ▼
    Write 204 No Content  next.ServeHTTP(w, r)  next.ServeHTTP(w, r)
    (Short-circuit early) (Pass to handlers)    (Pass to handlers)
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Preflight Short-Circuiting
HTTP `OPTIONS` requests sent by browsers prior to complex cross-origin calls (e.g. containing custom `Authorization` or `Content-Type` headers) do not carry business logic. Intercepting `OPTIONS` and returning `http.StatusNoContent` (`204`) immediately prevents unnecessary route matching, database hits, or authentication token parsing.

### 2. Case-Insensitive Origin Matching
Domain names are case-insensitive in DNS and HTTP standards. The middleware uses `strings.EqualFold(o, origin)` when verifying allowed origins to prevent bypasses caused by casing differences (`https://API.Example.com` vs `https://api.example.com`).

### 3. Dynamic Origin vs Wildcard `*`
When `Access-Control-Allow-Credentials: true` is required, the CORS specification prohibits the wildcard `*`. In secure configurations, the middleware reflects the exact validated incoming `Origin` value back in `Access-Control-Allow-Origin` rather than an indiscriminate wildcard.

### 4. Content Security Policy (CSP) Tuning
While `default-src 'self'` provides a robust baseline for pure REST/JSON APIs, frontend services rendering server-side HTML may require custom directives for CDNs, fonts, or inline scripts.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Implementation (`patterns/11_middleware/security_cors.go`)

```go
package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeadersMiddleware sets standard defensive browser security headers.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		h.Set("Content-Security-Policy", "default-src 'self'")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// CORSConfig specifies allowed cross-origin settings.
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAgeSeconds  int
}

// CORSMiddleware handles cross-origin requests and OPTIONS preflights.
func CORSMiddleware(cfg CORSConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				allowed := false
				for _, o := range cfg.AllowedOrigins {
					if o == "*" || strings.EqualFold(o, origin) {
						allowed = true
						w.Header().Set("Access-Control-Allow-Origin", origin)
						break
					}
				}

				if allowed {
					if len(cfg.AllowedMethods) > 0 {
						w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
					}
					if len(cfg.AllowedHeaders) > 0 {
						w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
					}
				}
			}

			// Intercept preflight OPTIONS request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

### 2. Integration into Production Server

```go
func BuildSecureRouter() http.Handler {
    corsCfg := middleware.CORSConfig{
        AllowedOrigins: []string{"https://app.example.com", "https://dashboard.example.com"},
        AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
        AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID"},
        MaxAgeSeconds:  86400,
    }

    chain := middleware.NewChain(
        middleware.SecurityHeadersMiddleware,
        middleware.CORSMiddleware(corsCfg),
    )

    mux := http.NewServeMux()
    mux.HandleFunc("/api/v1/profile", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"status":"ok"}`))
    })

    return chain.Then(mux)
}
```

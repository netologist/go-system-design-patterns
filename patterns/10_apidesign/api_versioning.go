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

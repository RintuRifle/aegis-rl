package middleware

import (
	"net"
	"net/http"
	"strings"
)

// extractIdentity determines the client's identity for rate limiting.
// Priority: X-API-Key header (most reliable) → spoof-safe IP extraction.
//
// trustProxy must only be true when the service sits behind a trusted reverse
// proxy (Caddy/Nginx). X-Forwarded-For and X-Real-IP are client-controlled
// headers — honoring them on a direct connection lets an attacker rotate
// identities (and thus reset their bucket) with a one-line curl flag.
func extractIdentity(r *http.Request, trustProxy bool) string {
	// 1. Check for API key header first — most reliable identity
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "key:" + apiKey
	}

	// 2. Fall back to IP
	return "ip:" + extractIP(r, trustProxy)
}

// extractIP returns the client's real IP address.
// Behind a trusted proxy: last entry in X-Forwarded-For (the one OUR proxy
// appended from the actual TCP connection), then X-Real-IP.
// Direct connection: RemoteAddr only — forwarded headers are ignored.
func extractIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// Trust only the LAST hop in XFF — earlier entries are client-supplied
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			lastIP := strings.TrimSpace(parts[len(parts)-1])
			if lastIP != "" {
				return lastIP
			}
		}

		// X-Real-IP (set by some proxies like Nginx)
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return realIP
		}
	}

	// RemoteAddr — the actual TCP peer, unforgeable
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Authorized reports whether the request carries a valid token matching feedToken.
// The token is accepted from the "token" query parameter or the Authorization
// header as a Bearer token, checked in that order.
// If feedToken is empty, all requests are allowed (intended for local development only).
func Authorized(r *http.Request, feedToken string) bool {
	// Empty token means the server is running in open/dev mode — allow everything.
	if feedToken == "" {
		return true
	}

	// Prefer the query parameter; fall back to the Authorization header.
	got := r.URL.Query().Get("token")
	if got == "" {
		got = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}

	// Use constant-time comparison to prevent timing-based token enumeration.
	return subtle.ConstantTimeCompare([]byte(got), []byte(feedToken)) == 1
}

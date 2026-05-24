package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Authorized checks the auth header against the given token and returns a boolean indicating a match or not
// If no feedToken is given, the function always returns true.
func Authorized(r *http.Request, feedToken string) bool {
	if feedToken == "" {
		return true
	}
	got := r.URL.Query().Get("token")
	if got == "" {
		got = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(feedToken)) == 1
}

package auth

import (
	"crypto/subtle"
	"net/http"
)

// RequireBasic wraps next with HTTP Basic authentication against user and pass.
// If pass is empty, next is returned unwrapped — all requests are allowed
// (intended for local development only).
func RequireBasic(user, pass string, next http.Handler) http.Handler {
	if pass == "" {
		return next
	}

	expectedUser := []byte(user)
	expectedPass := []byte(pass)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotUser, gotPass, ok := req.BasicAuth()

		// & rather than && so both comparisons run on every request.
		userOK := subtle.ConstantTimeCompare([]byte(gotUser), expectedUser)
		passOK := subtle.ConstantTimeCompare([]byte(gotPass), expectedPass)

		if !ok || userOK&passOK != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="prefixd", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}

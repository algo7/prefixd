// Package handler contains the HTTP handlers that wire together the auth and
// feed packages and serve the prefixd public endpoints.
package handler

import (
	"log"
	"net/http"

	"github.com/algo7/prefixd/internal/auth"
	"github.com/algo7/prefixd/internal/feed"
)

// Feed returns an http.HandlerFunc that authenticates the request with token
// and writes the merged prefix feed read from path. An empty token disables
// authentication, which is intended for local development only.
// The handler responds with 403 on auth failure and 500 if the feed cannot be
// read or parsed.
func Feed(token, path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Reject unauthenticated requests before doing any file I/O.
		if !auth.Authorized(r, token) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		body, err := feed.BuildFeed(path)
		if err != nil {
			log.Printf("build feed: %v", err)
			http.Error(w, "feed unavailable", http.StatusInternalServerError)
			return
		}

		// Serve as plain text so clients can consume the feed with simple line readers.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(body)
	}
}

// Healthz is a liveness probe that always returns 200 OK with the body "ok".
func Healthz(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

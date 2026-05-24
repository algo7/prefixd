package main

import (
	"log"
	"net/http"
	"os"

	"github.com/algo7/go-utils/bootstrap"
	"github.com/algo7/prefixd/internal/auth"
	"github.com/algo7/prefixd/internal/feed"
)

var (
	feedToken  = os.Getenv("FEED_TOKEN") // empty => open (dev only)
	feedFile   = bootstrap.GetEnvWithDefaultStr("FEED_FILE", "feed.txt")
	listenAddr = bootstrap.GetEnvWithDefaultStr("LISTEN_ADDR", ":8080")
)

func main() {
	http.HandleFunc("/feed", feedHandler)

	// /healthz is a simple liveness probe that always returns 200 OK.
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("serving %s on %s", feedFile, listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

// feedHandler authenticates the request and writes the current prefix feed to
// the response. It returns 403 if the token is missing or incorrect, and 500 if
// the feed file cannot be read or parsed.
func feedHandler(w http.ResponseWriter, r *http.Request) {
	// Reject unauthenticated requests before doing any file I/O.
	if !auth.Authorized(r, feedToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	body, err := feed.BuildFeed(feedFile)
	if err != nil {
		log.Printf("build feed: %v", err)
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return
	}

	// Serve as plain text so clients can consume the feed with simple line readers.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(body)
}

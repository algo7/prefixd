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
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	log.Printf("serving %s on %s", feedFile, listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func feedHandler(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(body)
}

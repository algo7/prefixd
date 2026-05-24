package main

import (
	"log"
	"net/http"
	"os"

	"github.com/algo7/go-utils/bootstrap"
	"github.com/algo7/prefixd/internal/handler"
)

var (
	feedToken  = os.Getenv("FEED_TOKEN") // empty => open (dev only)
	feedFile   = bootstrap.GetEnvWithDefaultStr("FEED_FILE", "feed.txt")
	listenAddr = bootstrap.GetEnvWithDefaultStr("LISTEN_ADDR", ":8080")
	Version    string
)

// main registers HTTP handlers and starts the server.
func main() {
	http.HandleFunc("/feed", handler.Feed(feedToken, feedFile))
	http.HandleFunc("/healthz", handler.Healthz)

	log.Printf("prefixd version %s serving %s on %s", Version, feedFile, listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

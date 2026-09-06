package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/algo7/go-utils/bootstrap"
	"github.com/algo7/prefixd/internal/auth"
	"github.com/algo7/prefixd/internal/config"
)

var (
	aliasUser  = bootstrap.GetEnvWithDefaultStr("ALIAS_USER", "pfsense")
	aliasPass  = os.Getenv("ALIAS_PASS") // empty => open (dev only)
	aliasFile  = bootstrap.GetEnvWithDefaultStr("ALIAS_FILE", "aliases.yaml")
	listenAddr = bootstrap.GetEnvWithDefaultStr("LISTEN_ADDR", ":8080")
	Version    string
)

// main registers HTTP handlers and starts the server.
func main() {

	// Load the config file
	c, err := config.New(aliasFile)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{name}", auth.RequireBasic(aliasUser, aliasPass, c))

	if aliasPass == "" {
		log.Printf("WARNING: ALIAS_PASS unset, aliases are served without authentication")
	}

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("prefixd version %s serving %s on %s", Version, aliasFile, listenAddr)
	log.Fatal(srv.ListenAndServe())
}

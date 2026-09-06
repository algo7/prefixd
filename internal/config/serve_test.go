package config

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// serve drives the alias handler for one alias name, bypassing the router so
// the handler's own behavior is what is under test.
func serve(t *testing.T, cfg *Config, name string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+name, nil)
	req.SetPathValue("name", name)
	rec := httptest.NewRecorder()
	cfg.ServeHTTP(rec, req)
	return rec
}

// TestServeHTTPWritesEntriesInOrder pins the wire format pfSense parses: one
// entry per line, in the order the config declares them.
func TestServeHTTPWritesEntriesInOrder(t *testing.T) {
	cfg := &Config{Aliases: map[string][]string{
		"colors": {"192.168.2.0/24", "192.168.1.0/24", "10.0.0.1"},
	}}

	rec := serve(t, cfg, "colors")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	const want = "192.168.2.0/24\n192.168.1.0/24\n10.0.0.1\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
}

// TestServeHTTPNotFound asserts that names with no alias behind them 404
// rather than returning an empty 200, which pfSense would load as an alias
// that silently matches nothing.
func TestServeHTTPNotFound(t *testing.T) {
	cfg := &Config{Aliases: map[string][]string{
		"colors": {"10.0.0.1"},
	}}

	tests := []struct {
		name  string
		alias string
	}{
		{name: "unknown alias", alias: "nope"},
		{name: "empty name", alias: ""},
		{name: "case differs from the configured name", alias: "COLORS"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(t, cfg, tc.alias)
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
			}
		})
	}
}

// TestServeHTTPEmptyConfig asserts a config with no aliases serves nothing
// rather than panicking on a nil map.
func TestServeHTTPEmptyConfig(t *testing.T) {
	rec := serve(t, &Config{}, "colors")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

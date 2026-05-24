package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFeedFile creates a temp file with the given content and returns its path.
func writeFeedFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "feed.txt")
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
	return path
}

// TestFeed covers the HTTP behavior of the Feed handler: authentication
// outcomes, successful feed delivery, and the error response when the feed
// file is missing.
func TestFeed(t *testing.T) {
	validFeed := writeFeedFile(t, "10.0.0.0/8\n")
	missingFile := filepath.Join(t.TempDir(), "nope.txt")

	tests := []struct {
		name        string
		serverToken string
		feedFile    string
		query       string
		header      string
		wantStatus  int
		wantBody    string
		wantCType   string
	}{
		{
			name:        "open mode serves the feed without a token",
			serverToken: "",
			feedFile:    validFeed,
			wantStatus:  http.StatusOK,
			wantBody:    "10.0.0.0/8\n",
			wantCType:   "text/plain; charset=utf-8",
		},
		{
			name:        "missing token returns 403 when server requires auth",
			serverToken: "secret",
			feedFile:    validFeed,
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "wrong token returns 403",
			serverToken: "secret",
			feedFile:    validFeed,
			query:       "wrong",
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "correct query token serves the feed",
			serverToken: "secret",
			feedFile:    validFeed,
			query:       "secret",
			wantStatus:  http.StatusOK,
			wantBody:    "10.0.0.0/8\n",
			wantCType:   "text/plain; charset=utf-8",
		},
		{
			name:        "correct Bearer header token serves the feed",
			serverToken: "secret",
			feedFile:    validFeed,
			header:      "Bearer secret",
			wantStatus:  http.StatusOK,
			wantBody:    "10.0.0.0/8\n",
			wantCType:   "text/plain; charset=utf-8",
		},
		{
			name:        "header token without Bearer prefix is rejected",
			serverToken: "secret",
			feedFile:    validFeed,
			header:      "secret",
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "missing feed file returns 500",
			serverToken: "",
			feedFile:    missingFile,
			wantStatus:  http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/feed"
			if tc.query != "" {
				url += "?token=" + tc.query
			}
			r := httptest.NewRequest(http.MethodGet, url, nil)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()

			Feed(tc.serverToken, tc.feedFile)(w, r)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantBody != "" {
				assert.Equal(t, tc.wantBody, w.Body.String())
			}
			if tc.wantCType != "" {
				assert.Equal(t, tc.wantCType, w.Header().Get("Content-Type"))
			}
		})
	}
}

// TestHealthz verifies the liveness probe always returns 200 OK with body "ok".
func TestHealthz(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	Healthz(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

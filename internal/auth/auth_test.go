package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// downstream returns a handler that records its invocation and answers with a
// sentinel status, so tests can tell "next ran" apart from anything the
// middleware itself writes.
func downstream(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*called = true
		w.WriteHeader(http.StatusTeapot)
	})
}

// TestRequireBasicAllowsValidCredentials covers the paths that must reach the
// wrapped handler.
func TestRequireBasicAllowsValidCredentials(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		pass     string
		sendAuth bool
		sendUser string
		sendPass string
	}{
		{
			name:     "empty password disables authentication",
			user:     "pfsense",
			pass:     "",
			sendAuth: false,
		},
		{
			name:     "matching credentials",
			user:     "pfsense",
			pass:     "s3cret",
			sendAuth: true,
			sendUser: "pfsense",
			sendPass: "s3cret",
		},
		{
			name:     "password containing colons is not truncated",
			user:     "pfsense",
			pass:     "s3:cr:et",
			sendAuth: true,
			sendUser: "pfsense",
			sendPass: "s3:cr:et",
		},
		{
			name:     "empty configured user matches an empty username",
			user:     "",
			pass:     "s3cret",
			sendAuth: true,
			sendUser: "",
			sendPass: "s3cret",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := RequireBasic(tc.user, tc.pass, downstream(&called))

			req := httptest.NewRequest(http.MethodGet, "/colors", nil)
			if tc.sendAuth {
				req.SetBasicAuth(tc.sendUser, tc.sendPass)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if !called {
				t.Error("wrapped handler was not reached")
			}
			if rec.Code != http.StatusTeapot {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
			}
		})
	}
}

// TestRequireBasicRejects covers every path that must produce a 401 without
// reaching the wrapped handler.
func TestRequireBasicRejects(t *testing.T) {
	const (
		user = "pfsense"
		pass = "s3cret"
	)

	tests := []struct {
		name    string
		prepare func(*http.Request)
	}{
		{
			name:    "no Authorization header",
			prepare: func(*http.Request) {},
		},
		{
			name:    "wrong user",
			prepare: func(r *http.Request) { r.SetBasicAuth("admin", pass) },
		},
		{
			name:    "wrong password",
			prepare: func(r *http.Request) { r.SetBasicAuth(user, "hunter2") },
		},
		{
			name:    "wrong user and password",
			prepare: func(r *http.Request) { r.SetBasicAuth("admin", "hunter2") },
		},
		{
			name:    "empty credentials",
			prepare: func(r *http.Request) { r.SetBasicAuth("", "") },
		},
		{
			name: "bearer token from the pre-basic-auth scheme",
			prepare: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+pass)
			},
		},
		{
			name: "undecodable basic credentials",
			prepare: func(r *http.Request) {
				r.Header.Set("Authorization", "Basic !!!not-base64!!!")
			},
		},
		{
			name:    "password sent as the username",
			prepare: func(r *http.Request) { r.SetBasicAuth(pass, user) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := RequireBasic(user, pass, downstream(&called))

			req := httptest.NewRequest(http.MethodGet, "/colors", nil)
			tc.prepare(req)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if called {
				t.Error("wrapped handler was reached on a rejected request")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			// Without a challenge, clients have nothing to retry with.
			challenge := rec.Header().Get("WWW-Authenticate")
			if !strings.HasPrefix(challenge, "Basic ") {
				t.Errorf("WWW-Authenticate = %q, want a Basic challenge", challenge)
			}
			if !strings.Contains(challenge, "realm=") {
				t.Errorf("WWW-Authenticate = %q, want a realm", challenge)
			}
		})
	}
}

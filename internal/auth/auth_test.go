package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAuthorized exercises Authorized across every supported token source
// (query, header) and every authorization outcome (open mode, match, mismatch).
func TestAuthorized(t *testing.T) {
	const secret = "s3cret-token"

	tests := []struct {
		name      string
		feedToken string
		query     string
		header    string
		want      bool
	}{
		{
			name:      "empty server token allows any request",
			feedToken: "",
			want:      true,
		},
		{
			name:      "empty server token allows request even when client sends a token",
			feedToken: "",
			query:     "anything",
			want:      true,
		},
		{
			name:      "matching token in query parameter is accepted",
			feedToken: secret,
			query:     secret,
			want:      true,
		},
		{
			name:      "matching Bearer token in Authorization header is accepted",
			feedToken: secret,
			header:    "Bearer " + secret,
			want:      true,
		},
		{
			name:      "wrong token in query parameter is rejected",
			feedToken: secret,
			query:     "wrong",
			want:      false,
		},
		{
			name:      "wrong Bearer token in header is rejected",
			feedToken: secret,
			header:    "Bearer wrong",
			want:      false,
		},
		{
			name:      "missing token entirely is rejected",
			feedToken: secret,
			want:      false,
		},
		{
			name:      "query token takes precedence over header token",
			feedToken: secret,
			query:     secret,
			header:    "Bearer wrong",
			want:      true,
		},
		{
			name:      "query token takes precedence even when it is wrong",
			feedToken: secret,
			query:     "wrong",
			header:    "Bearer " + secret,
			want:      false,
		},
		{
			name:      "raw header value without Bearer prefix is rejected",
			feedToken: secret,
			header:    secret,
			want:      false,
		},
		{
			name:      "lowercase bearer prefix is rejected (case-sensitive scheme)",
			feedToken: secret,
			header:    "bearer " + secret,
			want:      false,
		},
		{
			name:      "empty Bearer header value is rejected",
			feedToken: secret,
			header:    "Bearer ",
			want:      false,
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

			got := Authorized(r, tc.feedToken)
			assert.Equal(t, tc.want, got)
		})
	}
}

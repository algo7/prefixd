package feed

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFeed writes content to a temp file under t.TempDir() and returns its path.
// Using a per-test temp directory ensures tests cannot interfere with each other.
func writeFeed(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "feed.txt")
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
	return path
}

// resetCache clears the package-level feed cache between tests that need
// to start from a known state (i.e. tests that verify cache behavior).
func resetCache(t *testing.T) {
	t.Helper()
	cache.Lock()
	cache.path = ""
	cache.mtime = time.Time{}
	cache.data = nil
	cache.Unlock()
}

// TestBuildFeedParse covers parsing, normalization, merging and sorting of
// the input file. It exercises buildFeed directly to bypass the cache.
func TestBuildFeedParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "single bare IPv4 becomes a host route",
			content: "127.0.0.1\n",
			want:    "127.0.0.1/32\n",
		},
		{
			name:    "single bare IPv6 becomes a host route",
			content: "::1\n",
			want:    "::1/128\n",
		},
		{
			name:    "single IPv4 CIDR is preserved",
			content: "10.0.0.0/8\n",
			want:    "10.0.0.0/8\n",
		},
		{
			name:    "single IPv6 CIDR is preserved",
			content: "2001:db8::/32\n",
			want:    "2001:db8::/32\n",
		},
		{
			name:    "empty file yields empty output",
			content: "",
			want:    "",
		},
		{
			name:    "only comments and blank lines yields empty output",
			content: "# header comment\n\n   \n# another\n",
			want:    "",
		},
		{
			name:    "comments and blank lines are skipped around real entries",
			content: "# comment\n\n10.0.0.0/8\n# trailing\n",
			want:    "10.0.0.0/8\n",
		},
		{
			name:    "surrounding whitespace is trimmed",
			content: "   10.0.0.0/8   \n",
			want:    "10.0.0.0/8\n",
		},
		{
			name:    "invalid entries are skipped and valid entries remain",
			content: "not-an-ip\n10.0.0.0/8\nalso bad/zz\n",
			want:    "10.0.0.0/8\n",
		},
		{
			name:    "overlapping prefixes are merged into the larger one",
			content: "10.0.0.0/8\n10.1.0.0/16\n",
			want:    "10.0.0.0/8\n",
		},
		{
			name:    "adjacent prefixes are merged into a single supernet",
			content: "10.0.0.0/9\n10.128.0.0/9\n",
			want:    "10.0.0.0/8\n",
		},
		{
			name:    "duplicate prefixes are deduplicated",
			content: "10.0.0.0/8\n10.0.0.0/8\n",
			want:    "10.0.0.0/8\n",
		},
		{
			name:    "mixed IPv4 and IPv6 prefixes are both emitted",
			content: "10.0.0.0/8\n2001:db8::/32\n",
			want:    "10.0.0.0/8\n2001:db8::/32\n",
		},
		{
			name:    "output is sorted lexicographically",
			content: "192.168.0.0/16\n10.0.0.0/8\n172.16.0.0/12\n",
			want:    "10.0.0.0/8\n172.16.0.0/12\n192.168.0.0/16\n",
		},
		{
			name:    "bare IP and CIDR mix sorted correctly",
			content: "127.0.0.1\n10.0.0.0/8\n",
			want:    "10.0.0.0/8\n127.0.0.1/32\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFeed(t, tc.content)

			got, err := buildFeed(path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

// TestBuildFeedFileErrors verifies that missing or unreadable files produce
// errors rather than silently returning empty output.
func TestBuildFeedFileErrors(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "nonexistent file returns error",
			path: filepath.Join(t.TempDir(), "does-not-exist.txt"),
		},
		{
			name: "empty path returns error",
			path: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildFeed(tc.path)
			assert.Error(t, err)
		})
	}
}

// TestBuildFeedCacheHit verifies that an unchanged file is served from the
// cache rather than re-parsed. We mutate the file content while restoring its
// mtime so the cache MUST still match for the assertion to pass.
func TestBuildFeedCacheHit(t *testing.T) {
	resetCache(t)

	path := writeFeed(t, "10.0.0.0/8\n")

	first, err := BuildFeed(path)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/8\n", string(first))

	// Capture the file's current mtime so we can pin it back after rewriting.
	info, err := os.Stat(path)
	require.NoError(t, err)
	originalMtime := info.ModTime()

	// Rewrite the file with different content but restore the original mtime;
	// the cache should NOT detect this as a change.
	err = os.WriteFile(path, []byte("192.168.0.0/16\n"), 0o644)
	require.NoError(t, err)
	err = os.Chtimes(path, originalMtime, originalMtime)
	require.NoError(t, err)

	second, err := BuildFeed(path)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "cache should serve stale content when mtime is unchanged")
}

// TestBuildFeedCacheInvalidation verifies that advancing the file's mtime
// causes the next call to re-parse and return the new content.
func TestBuildFeedCacheInvalidation(t *testing.T) {
	resetCache(t)

	path := writeFeed(t, "10.0.0.0/8\n")

	first, err := BuildFeed(path)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/8\n", string(first))

	// Rewrite the file and explicitly advance its mtime so the cache invalidates.
	err = os.WriteFile(path, []byte("192.168.0.0/16\n"), 0o644)
	require.NoError(t, err)
	newer := time.Now().Add(1 * time.Second)
	err = os.Chtimes(path, newer, newer)
	require.NoError(t, err)

	second, err := BuildFeed(path)
	require.NoError(t, err)
	assert.Equal(t, "192.168.0.0/16\n", string(second), "cache should rebuild after mtime advances")
}

// TestBuildFeedCacheDifferentPath verifies that calling BuildFeed with a new
// path bypasses the cache (which only tracks one path at a time).
func TestBuildFeedCacheDifferentPath(t *testing.T) {
	resetCache(t)

	pathA := writeFeed(t, "10.0.0.0/8\n")
	pathB := writeFeed(t, "192.168.0.0/16\n")

	dataA, err := BuildFeed(pathA)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/8\n", string(dataA))

	dataB, err := BuildFeed(pathB)
	require.NoError(t, err)
	assert.Equal(t, "192.168.0.0/16\n", string(dataB), "different path should not return cached data from another path")
}

// TestBuildFeedStatError verifies that a missing file surfaces an error from
// the Stat call rather than being masked by the cache.
func TestBuildFeedStatError(t *testing.T) {
	resetCache(t)

	_, err := BuildFeed(filepath.Join(t.TempDir(), "missing.txt"))
	assert.Error(t, err)
}

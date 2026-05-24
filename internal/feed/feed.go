package feed

import (
	"bufio"
	"log"
	"net/netip"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go4.org/netipx"
)

// cache holds the most recently built feed so the file is not re-parsed on
// every request. It is invalidated when the file's mtime advances.
var cache struct {
	sync.RWMutex
	path  string
	mtime time.Time
	data  []byte
}

// BuildFeed returns the merged, deduplicated, and sorted prefix feed for the
// file at path. The result is cached; the file is only re-parsed when its
// modification time changes since the last call.
func BuildFeed(path string) ([]byte, error) {
	// Stat the file to get its current modification time for cache validation.
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	mtime := info.ModTime()

	// Fast path: return the cached feed if the file has not changed.
	cache.RLock()
	if cache.path == path && !mtime.After(cache.mtime) {
		data := cache.data
		cache.RUnlock()
		return data, nil
	}
	cache.RUnlock()

	// Slow path: file is new or has been updated, rebuild the feed.
	data, err := buildFeed(path)
	if err != nil {
		return nil, err
	}

	// Store the newly built feed and the mtime we compared against.
	cache.Lock()
	cache.path = path
	cache.mtime = mtime
	cache.data = data
	cache.Unlock()

	return data, nil
}

// buildFeed reads the file at path, parses each line as an IP address or CIDR
// prefix, merges overlapping ranges, and returns a newline-delimited sorted list
// of canonical prefixes.
func buildFeed(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// IPSetBuilder merges and deduplicates overlapping prefixes automatically.
	b := netipx.IPSetBuilder{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		// Skip blank lines and comment lines starting with '#'.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Lines without a '/' are bare IP addresses; convert them to host routes.
		if !strings.Contains(line, "/") {
			a, err := netip.ParseAddr(line)
			if err != nil {
				log.Printf("skip invalid entry %q: %v", line, err)
				continue
			}
			// Represent the single IP as a /32 (IPv4) or /128 (IPv6) prefix.
			b.AddPrefix(netip.PrefixFrom(a, a.BitLen()))
			continue
		}

		// Lines with a '/' are parsed as CIDR prefixes.
		p, err := netip.ParsePrefix(line)
		if err != nil {
			log.Printf("skip invalid entry %q: %v", line, err)
			continue
		}
		b.AddPrefix(p)
	}
	err = sc.Err()
	if err != nil {
		return nil, err
	}

	// Finalise the set; this performs the actual merge of overlapping ranges.
	set, err := b.IPSet()
	if err != nil {
		return nil, err
	}

	// Convert each prefix to its string representation for sorting and output.
	prefixes := set.Prefixes()
	lines := make([]string, len(prefixes))
	for i, p := range prefixes {
		lines[i] = p.String()
	}

	// Sort lexicographically so the output is deterministic and human-readable.
	sort.Strings(lines)

	// An empty input yields an empty output, not a stray newline.
	if len(lines) == 0 {
		return []byte{}, nil
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

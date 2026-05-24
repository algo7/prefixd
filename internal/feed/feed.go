package feed

import (
	"bufio"
	"log"
	"net/netip"
	"os"
	"sort"
	"strings"

	"go4.org/netipx"
)

// BuildFeed reads the text file and returns a merged, deduped, sorted feed.
func BuildFeed(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b := netipx.IPSetBuilder{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		// Skip empty or commented lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Accept a bare IP as a host route, otherwise parse as CIDR.
		if !strings.Contains(line, "/") {
			a, err := netip.ParseAddr(line)
			if err == nil {
				b.AddPrefix(netip.PrefixFrom(a, a.BitLen()))
				continue
			}
		}
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

	set, err := b.IPSet()
	if err != nil {
		return nil, err
	}

	prefixes := set.Prefixes()
	lines := make([]string, len(prefixes))
	for i, p := range prefixes {
		lines[i] = p.String()
	}
	sort.Strings(lines)

	sb := strings.Builder{}
	for _, l := range lines {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	return []byte(sb.String()), nil
}

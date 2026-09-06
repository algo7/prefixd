package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes body to a file in a temp dir and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aliases.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

// TestNewRejectsInvalidEntries asserts that a malformed entry aborts startup
// and that the error names both the offending alias and the offending value.
func TestNewRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string // fragments that must all appear in the error
	}{
		{
			name: "IPv4 prefix with host bits set",
			yaml: "bad:\n  - \"192.168.1.5/24\"\n",
			want: []string{`alias "bad"`, "host bits set", "192.168.1.0/24", "192.168.1.5/32"},
		},
		{
			name: "IPv6 prefix with host bits set",
			yaml: "bad:\n  - \"2001:db8::1/32\"\n",
			want: []string{`alias "bad"`, "host bits set", "2001:db8::/32", "2001:db8::1/128"},
		},
		{
			name: "prefix length out of range",
			yaml: "bad:\n  - \"192.168.1.0/33\"\n",
			want: []string{`alias "bad"`},
		},
		{
			name: "address in a prefix field",
			yaml: "bad:\n  - \"not-an-ip/24\"\n",
			want: []string{`alias "bad"`},
		},
		{
			name: "unparseable address",
			yaml: "bad:\n  - \"not-an-ip\"\n",
			want: []string{`alias "bad"`, "invalid IP", "not-an-ip"},
		},
		{
			name: "empty string entry",
			yaml: "bad:\n  - \"\"\n",
			want: []string{`alias "bad"`, "invalid IP"},
		},
		{
			name: "IPv6 address carrying a zone",
			yaml: "bad:\n  - \"fe80::1%eth0\"\n",
			want: []string{`alias "bad"`, "zone suffix not allowed"},
		},
		{
			name: "hostname instead of an address",
			yaml: "bad:\n  - \"example.com\"\n",
			want: []string{`alias "bad"`, "invalid IP"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := New(writeConfig(t, tc.yaml))
			if err == nil {
				t.Fatalf("New() succeeded, want an error; got %+v", cfg)
			}
			if cfg != nil {
				t.Errorf("New() returned a config alongside its error: %+v", cfg)
			}
			for _, frag := range tc.want {
				if !strings.Contains(err.Error(), frag) {
					t.Errorf("error %q is missing %q", err, frag)
				}
			}
		})
	}
}

// TestNewReportsTheFirstBadEntry pins the entry the error points at. Entries
// are a slice, so the first invalid one is the one an operator should be sent
// to; reporting a later one would send them to the wrong line.
func TestNewReportsTheFirstBadEntry(t *testing.T) {
	_, err := New(writeConfig(t, "bad:\n  - \"first-bad\"\n  - \"second-bad\"\n"))
	if err == nil {
		t.Fatal("New() succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "first-bad") {
		t.Errorf("error %q does not name the first bad entry", err)
	}
	if strings.Contains(err.Error(), "second-bad") {
		t.Errorf("error %q names a later entry than the first bad one", err)
	}
}

// TestNewRejectsUnreadableConfig covers the two failures that happen before
// any entry is examined.
func TestNewRejectsUnreadableConfig(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := New(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
		if err == nil {
			t.Fatal("New() succeeded on a missing file, want an error")
		}
		if !strings.Contains(err.Error(), "failed to open config file") {
			t.Errorf("error %q does not identify the open failure", err)
		}
	})

	t.Run("malformed YAML", func(t *testing.T) {
		_, err := New(writeConfig(t, "colors:\n  - \"10.0.0.0/8\"\n neither indented nor a list\n"))
		if err == nil {
			t.Fatal("New() succeeded on malformed YAML, want an error")
		}
		if !strings.Contains(err.Error(), "failed to parse config file") {
			t.Errorf("error %q does not identify the parse failure", err)
		}
	})

	t.Run("scalar document instead of a mapping", func(t *testing.T) {
		_, err := New(writeConfig(t, "just-a-string\n"))
		if err == nil {
			t.Fatal("New() succeeded on a scalar document, want an error")
		}
		if !strings.Contains(err.Error(), "failed to parse config file") {
			t.Errorf("error %q does not identify the parse failure", err)
		}
	})
}

// TestNewAcceptsValidEntries asserts the forms an operator is allowed to use
// survive validation with their order and content intact.
func TestNewAcceptsValidEntries(t *testing.T) {
	const yaml = `colors:
  - "192.168.1.0/24"
  - "192.168.0.0/18"
  - "10.0.0.1"
hosts:
  - "10.0.0.5/32"
  - "2001:db8::/32"
  - "2001:db8::1/128"
  - "2001:db8::1"
  - "0.0.0.0/0"
  - "::/0"
`

	cfg, err := New(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("New() failed on a valid config: %v", err)
	}

	want := map[string][]string{
		"colors": {"192.168.1.0/24", "192.168.0.0/18", "10.0.0.1"},
		"hosts":  {"10.0.0.5/32", "2001:db8::/32", "2001:db8::1/128", "2001:db8::1", "0.0.0.0/0", "::/0"},
	}

	if len(cfg.Aliases) != len(want) {
		t.Fatalf("loaded %d aliases (%v), want %d", len(cfg.Aliases), cfg.Aliases, len(want))
	}
	for name, wantEntries := range want {
		got, ok := cfg.Aliases[name]
		if !ok {
			t.Errorf("alias %q is missing", name)
			continue
		}
		if len(got) != len(wantEntries) {
			t.Errorf("alias %q = %v, want %v", name, got, wantEntries)
			continue
		}
		for i := range wantEntries {
			if got[i] != wantEntries[i] {
				t.Errorf("alias %q entry %d = %q, want %q", name, i, got[i], wantEntries[i])
			}
		}
	}
}

// TestNewDropsEmptyAliases documents that an alias declared with no entries is
// discarded rather than rejected, so it 404s at request time instead of
// failing startup.
func TestNewDropsEmptyAliases(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "null value", yaml: "colors:\n  - \"10.0.0.0/8\"\nempty:\n"},
		{name: "explicit empty list", yaml: "colors:\n  - \"10.0.0.0/8\"\nempty: []\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := New(writeConfig(t, tc.yaml))
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}
			if _, ok := cfg.Aliases["empty"]; ok {
				t.Error(`alias "empty" was kept, want it dropped`)
			}
			if _, ok := cfg.Aliases["colors"]; !ok {
				t.Error(`alias "colors" was dropped alongside the empty one`)
			}
		})
	}
}

// TestNewAcceptsAnEmptyFile asserts an empty config loads rather than crashing
// the server at boot.
func TestNewAcceptsAnEmptyFile(t *testing.T) {
	cfg, err := New(writeConfig(t, ""))
	if err != nil {
		t.Fatalf("New() failed on an empty file: %v", err)
	}
	if len(cfg.Aliases) != 0 {
		t.Errorf("loaded %d aliases from an empty file, want 0", len(cfg.Aliases))
	}
}

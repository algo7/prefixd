package config

import (
	"fmt"
	"net/netip"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Aliases map[string][]string `yaml:",inline"`
}

// New loads and validates the aliases config.
func New(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}

	cfg := &Config{}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	for name, entries := range cfg.Aliases {
		if len(entries) == 0 {
			delete(cfg.Aliases, name)
			continue
		}

		for _, entry := range entries {
			if strings.Contains(entry, "/") {
				p, err := netip.ParsePrefix(entry)
				if err != nil {
					return nil, fmt.Errorf("alias %q: %w", name, err)
				}
				m := p.Masked()
				if p != m {
					return nil, fmt.Errorf("alias %q: %q has host bits set: use %s for the network or %s/%d for the single host",
						name, entry, m, p.Addr(), p.Addr().BitLen())
				}
				continue
			}
			a, err := netip.ParseAddr(entry)
			if err != nil {
				return nil, fmt.Errorf("alias %q: invalid IP %q: %w", name, entry, err)
			}
			if a.Zone() != "" {
				return nil, fmt.Errorf("alias %q: %q: zone suffix not allowed", name, entry)
			}
		}
	}
	return cfg, nil
}

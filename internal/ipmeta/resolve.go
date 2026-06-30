package ipmeta

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Region struct {
	Country     string `json:"country" yaml:"country"`
	Province    string `json:"province" yaml:"province"`
	City        string `json:"city" yaml:"city"`
	DisplayName string `json:"display_name" yaml:"display_name"`
	Source      string `json:"source" yaml:"source"`
	Custom      bool   `json:"custom" yaml:"custom"`
}

type cidrEntry struct {
	prefix  netip.Prefix
	region  Region
}

type Catalog struct {
	entries []cidrEntry
}

func (c *Catalog) Resolve(ip netip.Addr) (Region, bool) {
	if c == nil {
		return Region{}, false
	}
	for _, e := range c.entries {
		if e.prefix.Contains(ip) {
			return e.region, true
		}
	}
	return Region{}, false
}

func (c *Catalog) sort() {
	sort.Slice(c.entries, func(i, j int) bool {
		return c.entries[i].prefix.Bits() > c.entries[j].prefix.Bits()
	})
}

type Resolver struct {
	builtin *Catalog
	custom  *Catalog
}

func NewResolver(builtin, custom *Catalog) *Resolver {
	return &Resolver{builtin: builtin, custom: custom}
}

func (r *Resolver) Resolve(ip netip.Addr) (Region, bool) {
	if r == nil {
		return Region{}, false
	}
	if r.custom != nil {
		if region, ok := r.custom.Resolve(ip); ok {
			return region, true
		}
	}
	if r.builtin != nil {
		if region, ok := r.builtin.Resolve(ip); ok {
			return region, true
		}
	}
	return Region{}, false
}

type builtinEntry struct {
	CIDR    string `json:"cidr"`
	Country string `json:"country"`
	Province string `json:"province"`
	City    string `json:"city"`
}

func LoadBuiltin(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read builtin: %w", err)
	}
	var entries []builtinEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse builtin: %w", err)
	}
	c := &Catalog{}
	for _, e := range entries {
		prefix, err := netip.ParsePrefix(e.CIDR)
		if err != nil {
			continue
		}
		display := e.Country
		if e.Province != "" {
			display = e.Province
			if e.City != "" {
				display = e.City
			}
		}
		c.entries = append(c.entries, cidrEntry{
			prefix: prefix,
			region: Region{
				Country:     e.Country,
				Province:    e.Province,
				City:        e.City,
				DisplayName: display,
				Source:      "builtin",
			},
		})
	}
	c.sort()
	return c, nil
}

type customEntry struct {
	CIDR     string `yaml:"cidr"`
	Name     string `yaml:"name"`
	Country  string `yaml:"country"`
}

func LoadCustom(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Catalog{}, nil
		}
		return nil, fmt.Errorf("read custom: %w", err)
	}
	var entries []customEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse custom: %w", err)
	}
	c := &Catalog{}
	for _, e := range entries {
		prefix, err := netip.ParsePrefix(e.CIDR)
		if err != nil {
			continue
		}
		display := e.Name
		if display == "" {
			display = e.Country
		}
		c.entries = append(c.entries, cidrEntry{
			prefix: prefix,
			region: Region{
				Country:     e.Country,
				DisplayName: display,
				Source:      "custom",
				Custom:      true,
			},
		})
	}
	c.sort()
	return c, nil
}

func DisplayName(r Region) string {
	if r.DisplayName != "" {
		return r.DisplayName
	}
	parts := []string{r.Country, r.Province, r.City}
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "未知"
	}
	return strings.Join(out, " ")
}

package ipmeta

import (
	"os"
	"path/filepath"
	"testing"

	"net/netip"
)

func mustCatalog(t *testing.T, path string) *Catalog {
	t.Helper()
	c, err := LoadBuiltin(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustCustom(t *testing.T, path string) *Catalog {
	t.Helper()
	c, err := LoadCustom(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCustomCIDROverridesBuiltinGeo(t *testing.T) {
	dir := t.TempDir()
	builtinPath := filepath.Join(dir, "ipmeta-small.json")
	customPath := filepath.Join(dir, "custom-ranges.yaml")

	writeFixture(t, builtinPath, `[
		{"cidr": "58.216.0.0/16", "country": "中国", "province": "江苏", "city": "南京"},
		{"cidr": "140.205.0.0/16", "country": "中国", "province": "浙江", "city": "杭州"}
	]`)
	writeFixture(t, customPath, `- cidr: "58.216.48.0/24"
  name: "出口公网IP"
  country: "自定义"
`)

	builtin := mustCatalog(t, builtinPath)
	custom := mustCustom(t, customPath)
	resolver := NewResolver(builtin, custom)

	region, ok := resolver.Resolve(netip.MustParseAddr("58.216.48.6"))
	if !ok {
		t.Fatal("no region")
	}
	if region.DisplayName != "出口公网IP" {
		t.Fatalf("display = %q, want 出口公网IP", region.DisplayName)
	}
	if !region.Custom {
		t.Fatal("should be custom region")
	}

	region2, ok := resolver.Resolve(netip.MustParseAddr("140.205.70.178"))
	if !ok {
		t.Fatal("no region for builtin ip")
	}
	if region2.DisplayName != "杭州" {
		t.Fatalf("builtin display = %q, want 杭州", region2.DisplayName)
	}
}

func TestResolveUnknownIP(t *testing.T) {
	dir := t.TempDir()
	builtinPath := filepath.Join(dir, "ipmeta-small.json")
	writeFixture(t, builtinPath, `[
		{"cidr": "10.0.0.0/8", "country": "内网", "province": "", "city": ""}
	]`)

	builtin := mustCatalog(t, builtinPath)
	resolver := NewResolver(builtin, nil)

	_, ok := resolver.Resolve(netip.MustParseAddr("1.2.3.4"))
	if ok {
		t.Fatal("should not resolve unknown ip")
	}
}

func TestLoadCustomMissingFile(t *testing.T) {
	c, err := LoadCustom("/nonexistent/path/file.yaml")
	if err != nil {
		t.Fatalf("should return empty catalog for missing file: %v", err)
	}
	if c == nil {
		t.Fatal("catalog should not be nil")
	}
}

func TestDisplayNameFallback(t *testing.T) {
	r := Region{Country: "中国", Province: "北京", City: ""}
	got := DisplayName(r)
	if got != "中国 北京" {
		t.Fatalf("display = %q", got)
	}

	r2 := Region{}
	if DisplayName(r2) != "未知" {
		t.Fatalf("empty display = %q", DisplayName(r2))
	}
}

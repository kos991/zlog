package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergesDefaultsAndFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
server:
  listen: "0.0.0.0:9090"
paths:
  clickhouse_url: "http://localhost:8123"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Listen != "0.0.0.0:9090" {
		t.Fatalf("listen = %q", cfg.Server.Listen)
	}
	if cfg.Query.DefaultPageSize != 100 {
		t.Fatalf("default page size = %d", cfg.Query.DefaultPageSize)
	}
	if cfg.Storage.OrderBy != "log_date,dst_ip,ts" {
		t.Fatalf("order by = %q", cfg.Storage.OrderBy)
	}
	if cfg.Import.BatchSize != 5000 {
		t.Fatalf("batch size = %d", cfg.Import.BatchSize)
	}
}

func TestLoadDefaultsWhenNoFile(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Query.MaxRangeDays != 365 {
		t.Fatalf("max range days = %d", cfg.Query.MaxRangeDays)
	}
	if cfg.Storage.PartitionGranularity != "month" {
		t.Fatalf("partition granularity = %q", cfg.Storage.PartitionGranularity)
	}
}

func TestLoadRejectsInvalidPageSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
query:
  default_page_size: 0
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected error for invalid page size")
	}
}

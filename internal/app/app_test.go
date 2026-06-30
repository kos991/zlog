package app

import (
	"testing"

	"sangfor-log-search/internal/config"
)

func TestNewCreatesExportDir(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Paths.ExportDir = t.TempDir() + "/exports"
	cfg.Paths.AppLog = ""

	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_ = app
}

func TestBuildResolverReturnsErrorWhenNoData(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Paths.AppLog = ""
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = app.buildResolver()
	if err == nil {
		t.Fatal("expected error when no ipmeta data")
	}
}

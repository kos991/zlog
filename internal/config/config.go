package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Listen        string `yaml:"listen"`
		SessionSecret string `yaml:"session_secret"`
	} `yaml:"server"`
	Auth struct {
		AdminUsername     string `yaml:"admin_username"`
		AdminPasswordHash string `yaml:"admin_password_hash"`
	} `yaml:"auth"`
	Paths struct {
		ClickHouseURL string `yaml:"clickhouse_url"`
		ExportDir     string `yaml:"export_dir"`
		AppLog        string `yaml:"app_log"`
		LogDir        string `yaml:"log_dir"`
	} `yaml:"paths"`
	Import struct {
		ScanIntervalSeconds int `yaml:"scan_interval_seconds"`
		BatchSize           int `yaml:"batch_size"`
	} `yaml:"import"`
	Query struct {
		RequireTimeRange bool `yaml:"require_time_range"`
		MaxRangeDays     int  `yaml:"max_range_days"`
		DefaultPageSize  int  `yaml:"default_page_size"`
		MaxPageSize      int  `yaml:"max_page_size"`
	} `yaml:"query"`
	Storage struct {
		Compression         string `yaml:"compression"`
		PartitionGranularity string `yaml:"partition_granularity"`
		OrderBy             string `yaml:"order_by"`
	} `yaml:"storage"`
	Export struct {
		MaxRows       int `yaml:"max_rows"`
		RetentionDays int `yaml:"retention_days"`
	} `yaml:"export"`
}

func DefaultConfig() Config {
	var cfg Config
	cfg.Server.Listen = "0.0.0.0:8080"
	cfg.Paths.ClickHouseURL = "http://clickhouse:8123"
	cfg.Paths.ExportDir = "/var/lib/zlog/exports"
	cfg.Paths.AppLog = "/var/log/zlog/zlog.log"
	cfg.Paths.LogDir = "/data/sangfor_fw_log"
	cfg.Import.ScanIntervalSeconds = 300
	cfg.Import.BatchSize = 5000
	cfg.Query.RequireTimeRange = true
	cfg.Query.MaxRangeDays = 365
	cfg.Query.DefaultPageSize = 100
	cfg.Query.MaxPageSize = 500
	cfg.Storage.Compression = "zstd3"
	cfg.Storage.PartitionGranularity = "month"
	cfg.Storage.OrderBy = "log_date,dst_ip,ts"
	cfg.Export.MaxRows = 1000000
	cfg.Export.RetentionDays = 7
	cfg.Auth.AdminUsername = "admin"
	return cfg
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Server.Listen == "" {
		return fmt.Errorf("server.listen is required")
	}
	if cfg.Paths.ClickHouseURL == "" {
		return fmt.Errorf("paths.clickhouse_url is required")
	}
	if cfg.Query.DefaultPageSize <= 0 {
		return fmt.Errorf("query.default_page_size must be > 0")
	}
	if cfg.Import.BatchSize <= 0 {
		return fmt.Errorf("import.batch_size must be > 0")
	}
	return nil
}

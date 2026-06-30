package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"sangfor-log-search/internal/auth"
	"sangfor-log-search/internal/config"
	"sangfor-log-search/internal/exporter"
	http2 "sangfor-log-search/internal/http"
	"sangfor-log-search/internal/importer"
	"sangfor-log-search/internal/ipmeta"
	"sangfor-log-search/internal/jobs"
	"sangfor-log-search/internal/model"
	"sangfor-log-search/internal/store"
)

type App struct {
	cfg     config.Config
	logger  *log.Logger
	store   *store.ClickHouseStore
	server  *http.Server
}

func New(cfg config.Config) (*App, error) {
	logger := log.New(os.Stderr, "[zlog] ", log.LstdFlags|log.Lshortfile)
	if cfg.Paths.AppLog != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Paths.AppLog), 0755); err != nil {
			return nil, fmt.Errorf("create log dir: %w", err)
		}
		f, err := os.OpenFile(cfg.Paths.AppLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("open app log: %w", err)
		}
		logger = log.New(f, "[zlog] ", log.LstdFlags|log.Lshortfile)
	}
	return &App{cfg: cfg, logger: logger}, nil
}

func (a *App) Run(ctx context.Context) error {
	if err := os.MkdirAll(a.cfg.Paths.ExportDir, 0755); err != nil {
		return fmt.Errorf("create export dir: %w", err)
	}

	chStore, err := store.Open(ctx, a.cfg.Paths.ClickHouseURL)
	if err != nil {
		return fmt.Errorf("open clickhouse: %w", err)
	}
	defer chStore.Close()
	a.store = chStore

	if err := chStore.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	a.logger.Printf("clickhouse schema ready")

	resolver, err := a.buildResolver()
	if err != nil {
		a.logger.Printf("warning: ipmeta resolver not available: %v", err)
	}

	importerInst := importer.NewImporter(chStore, resolverAdapter{r: resolver}, a.cfg.Import.BatchSize)

	jobStore := jobs.NewClickHouseJobStore(chStore.Conn())

	exporterInst := exporter.New(
		jobStore,
		chStore.Query,
		chStore.Count,
		a.cfg.Paths.ExportDir,
		a.cfg.Export.MaxRows,
	)

	sessions := auth.NewSessionManager(a.cfg.Server.SessionSecret)

	deps := http2.Deps{
		Cfg:      a.cfg,
		Store:    chStore,
		Importer: importerInst,
		Exporter: exporterInst,
		JobStore: jobStore,
		Resolver: resolver,
		Sessions: sessions,
	}

	router := http2.NewRouter(deps)
	a.server = &http.Server{
		Addr:    a.cfg.Server.Listen,
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
	}()

	a.logger.Printf("zlog starting, listen=%s clickhouse=%s", a.cfg.Server.Listen, a.cfg.Paths.ClickHouseURL)
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	a.logger.Printf("zlog stopped")
	return nil
}

func (a *App) buildResolver() (*ipmeta.Resolver, error) {
	var builtin *ipmeta.Catalog
	var custom *ipmeta.Catalog

	builtinPath := os.Getenv("ZLOG_IPMETA_BUILTIN")
	if builtinPath == "" {
		builtinPath = "/opt/zlog/ipmeta-builtin.json"
	}
	if _, err := os.Stat(builtinPath); err == nil {
		b, err := ipmeta.LoadBuiltin(builtinPath)
		if err != nil {
			return nil, err
		}
		builtin = b
	}

	customPath := os.Getenv("ZLOG_IPMETA_CUSTOM")
	if customPath == "" {
		customPath = "/opt/zlog/custom-ranges.yaml"
	}
	if _, err := os.Stat(customPath); err == nil {
		c, err := ipmeta.LoadCustom(customPath)
		if err != nil {
			return nil, err
		}
		custom = c
	}

	if builtin == nil && custom == nil {
		return nil, fmt.Errorf("no ipmeta data found")
	}
	return ipmeta.NewResolver(builtin, custom), nil
}

type resolverAdapter struct {
	r *ipmeta.Resolver
}

func (a resolverAdapter) Resolve(row model.LogRow) string {
	if a.r == nil {
		return ""
	}
	region, ok := a.r.Resolve(row.DstIP)
	if !ok {
		return ""
	}
	return ipmeta.DisplayName(region)
}

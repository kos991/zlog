package exporter

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sangfor-log-search/internal/jobs"
	"sangfor-log-search/internal/model"
	"sangfor-log-search/internal/query"
)

type QueryFunc func(ctx context.Context, f query.LogFilter, limit, offset int) ([]model.LogRow, error)
type CountFunc func(ctx context.Context, f query.LogFilter) (uint64, error)

type Exporter struct {
	store    jobs.JobStore
	queryFn  QueryFunc
	countFn  CountFunc
	exportDir string
	maxRows  int
	mu       sync.Mutex
	pending  map[string]bool
}

func New(store jobs.JobStore, queryFn QueryFunc, countFn CountFunc, exportDir string, maxRows int) *Exporter {
	if maxRows <= 0 {
		maxRows = 1000000
	}
	return &Exporter{
		store:    store,
		queryFn:  queryFn,
		countFn:  countFn,
		exportDir: exportDir,
		maxRows:  maxRows,
		pending:  make(map[string]bool),
	}
}

func (e *Exporter) EnqueueCSV(ctx context.Context, f query.LogFilter, format string) (string, error) {
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "log" {
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	jobID := jobs.NewID()
	job := jobs.Job{
		ID:     jobID,
		Type:   "export_" + format,
		Status: "queued",
		Source: "api",
	}
	if err := e.store.Create(ctx, job); err != nil {
		return "", fmt.Errorf("create job: %w", err)
	}

	go e.runExport(jobID, f, format)
	return jobID, nil
}

func (e *Exporter) runExport(jobID string, f query.LogFilter, format string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	total, err := e.countFn(ctx, f)
	if err != nil {
		e.failJob(jobID, err)
		return
	}
	if total == 0 {
		e.failJob(jobID, fmt.Errorf("no rows to export"))
		return
	}
	if int(total) > e.maxRows {
		e.failJob(jobID, fmt.Errorf("too many rows: %d > %d", total, e.maxRows))
		return
	}

	e.updateJob(jobID, "running", 0, total, "")

	rows, err := e.queryFn(ctx, f, int(total), 0)
	if err != nil {
		e.failJob(jobID, err)
		return
	}

	ext := ".csv"
	if format == "log" {
		ext = ".log"
	}
	filename := fmt.Sprintf("export_%s%s", jobID, ext)
	path := filepath.Join(e.exportDir, filename)

	if err := os.MkdirAll(e.exportDir, 0755); err != nil {
		e.failJob(jobID, err)
		return
	}

	if format == "csv" {
		err = writeCSV(path, rows)
	} else {
		err = writeLog(path, rows)
	}
	if err != nil {
		e.failJob(jobID, err)
		return
	}

	e.updateJobWithResult(jobID, "done", uint64(len(rows)), total, "", path)
}

func (e *Exporter) updateJob(jobID, status string, progress, total uint64, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	job, err := e.store.Get(ctx, jobID)
	if err != nil {
		return
	}
	job.Status = status
	job.Progress = progress
	job.Total = total
	job.Error = errMsg
	_ = e.store.Update(ctx, job)
}

func (e *Exporter) updateJobWithResult(jobID, status string, progress, total uint64, errMsg, resultPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	job, err := e.store.Get(ctx, jobID)
	if err != nil {
		return
	}
	job.Status = status
	job.Progress = progress
	job.Total = total
	job.Error = errMsg
	job.ResultPath = resultPath
	_ = e.store.Update(ctx, job)
}

func (e *Exporter) failJob(jobID string, err error) {
	e.updateJob(jobID, "failed", 0, 0, err.Error())
}

func (e *Exporter) RunWorker(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

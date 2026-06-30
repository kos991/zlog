package importer

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sangfor-log-search/internal/model"
	"sangfor-log-search/internal/parser"
)

type Store interface {
	InsertBatch(ctx context.Context, rows []model.LogRow) error
	RecordImportState(ctx context.Context, sourceFile, fileHash string, rows, failedLines uint64, status string) error
	IsFileImported(ctx context.Context, sourceFile string) (bool, error)
	DeleteFileRows(ctx context.Context, sourceFile string) error
}

type Resolver interface {
	Resolve(ip model.LogRow) string
}

type ImportReport struct {
	SourceFile  string
	FileHash    string
	Rows        int
	FailedLines int
	Skipped     int
	StartedAt   time.Time
	FinishedAt  time.Time
	Error       string
}

type Importer struct {
	store     Store
	resolver  Resolver
	batchSize int
}

func NewImporter(store Store, resolver Resolver, batchSize int) *Importer {
	if batchSize <= 0 {
		batchSize = 5000
	}
	return &Importer{store: store, resolver: resolver, batchSize: batchSize}
}

func (i *Importer) ImportFile(ctx context.Context, path string, force bool) (ImportReport, error) {
	report := ImportReport{
		SourceFile: filepath.Base(path),
		StartedAt:  time.Now(),
	}
	defer func() {
		report.FinishedAt = time.Now()
	}()

	meta, err := parser.ParseFilename(filepath.Base(path))
	if err != nil {
		report.Error = err.Error()
		return report, err
	}

	if !force {
		already, err := i.store.IsFileImported(ctx, meta.SourceFile)
		if err != nil {
			report.Error = err.Error()
			return report, err
		}
		if already {
			report.Skipped = 1
			return report, nil
		}
	} else {
		if err := i.store.DeleteFileRows(ctx, meta.SourceFile); err != nil {
			report.Error = err.Error()
			return report, err
		}
	}

	hash, reader, err := openAndHash(path)
	if err != nil {
		report.Error = err.Error()
		return report, err
	}
	defer reader.Close()
	report.FileHash = hash

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	lineNo := uint32(0)
	batch := make([]model.LogRow, 0, i.batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := i.store.InsertBatch(ctx, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		row, err := parser.ParseLine(meta, line, lineNo)
		if err != nil {
			report.FailedLines++
			continue
		}
		if i.resolver != nil {
			row.DstCountry = i.resolver.Resolve(row)
		}
		batch = append(batch, row)
		report.Rows++
		if len(batch) >= i.batchSize {
			if err := flush(); err != nil {
				report.Error = err.Error()
				return report, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		report.Error = err.Error()
		return report, err
	}
	if err := flush(); err != nil {
		report.Error = err.Error()
		return report, err
	}

	if err := i.store.RecordImportState(ctx, meta.SourceFile, hash, uint64(report.Rows), uint64(report.FailedLines), "ok"); err != nil {
		report.Error = err.Error()
		return report, err
	}

	return report, nil
}

func (i *Importer) ImportDirectory(ctx context.Context, dir string) ([]ImportReport, error) {
	patterns := []string{"*.log", "*.log.gz", "*.log-*.gz"}
	var paths []string
	for _, p := range patterns {
		matched, err := filepath.Glob(filepath.Join(dir, p))
		if err != nil {
			return nil, err
		}
		paths = append(paths, matched...)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no log files found in %s", dir)
	}

	reports := make([]ImportReport, 0, len(paths))
	for _, path := range paths {
		report, err := i.ImportFile(ctx, path, false)
		if err != nil {
			report.Error = err.Error()
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func openAndHash(path string) (string, io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}

	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			_ = file.Close()
			return "", nil, err
		}
		return hashStream(file), &readCloser{r: gzReader, closers: []io.Closer{gzReader, file}}, nil
	}

	return hashStream(file), &readCloser{r: file, closers: []io.Closer{file}}, nil
}

func hashStream(r io.Reader) string {
	data, _ := io.ReadAll(r)
	sum := sha256.Sum256(data)
	_ = data
	return hex.EncodeToString(sum[:])
}

type readCloser struct {
	r       io.Reader
	closers []io.Closer
}

func (rc *readCloser) Read(p []byte) (int, error) {
	return rc.r.Read(p)
}

func (rc *readCloser) Close() error {
	var first error
	for _, c := range rc.closers {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

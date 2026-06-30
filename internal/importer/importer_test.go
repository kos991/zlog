package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sangfor-log-search/internal/model"
)

type fakeStore struct {
	imported   map[string]bool
	rows       []model.LogRow
	states     []fakeState
	deleted    []string
}

type fakeState struct {
	sourceFile  string
	fileHash    string
	rows        uint64
	failedLines uint64
	status      string
}

func newFakeStore() *fakeStore {
	return &fakeStore{imported: make(map[string]bool)}
}

func (s *fakeStore) InsertBatch(ctx context.Context, rows []model.LogRow) error {
	s.rows = append(s.rows, rows...)
	return nil
}

func (s *fakeStore) RecordImportState(ctx context.Context, sourceFile, fileHash string, rows, failedLines uint64, status string) error {
	s.states = append(s.states, fakeState{sourceFile, fileHash, rows, failedLines, status})
	if status == "ok" {
		s.imported[sourceFile] = true
	}
	return nil
}

func (s *fakeStore) IsFileImported(ctx context.Context, sourceFile string) (bool, error) {
	return s.imported[sourceFile], nil
}

func (s *fakeStore) DeleteFileRows(ctx context.Context, sourceFile string) error {
	s.deleted = append(s.deleted, sourceFile)
	s.imported[sourceFile] = false
	return nil
}

type fakeResolver struct{}

func (fakeResolver) Resolve(ip model.LogRow) string {
	return "TestCountry"
}

func writeSampleGZ(t *testing.T, path string, lines []string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	for _, line := range lines {
		gw.Write([]byte(line + "\n"))
	}
	gw.Close()
}

func TestImportFileCountsBadLinesWithoutStopping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "10.10.10.1_2026-04-28.log-20260429.gz")
	writeSampleGZ(t, path, []string{
		"Apr 28 00:00:23 localhost nat: 日志类型:NAT日志, NAT类型:snat, 源IP:2.55.81.106, 源端口:1799, 目的IP:140.205.70.178, 目的端口:443, 协议:6, 转换后的IP:58.216.48.6, 转换后的端口:1799",
		"this is a bad line without nat prefix",
		"Apr 28 00:00:24 localhost nat: 日志类型:NAT日志, NAT类型:dnat, 源IP:1.1.1.1, 源端口:80, 目的IP:2.2.2.2, 目的端口:443, 协议:6, 转换后的IP:3.3.3.3, 转换后的端口:80",
	})

	store := newFakeStore()
	imp := NewImporter(store, fakeResolver{}, 10)

	report, err := imp.ImportFile(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows == 0 || report.FailedLines == 0 {
		t.Fatalf("report = %+v", report)
	}
	if report.Rows != 2 {
		t.Fatalf("rows = %d, want 2", report.Rows)
	}
	if report.FailedLines != 1 {
		t.Fatalf("failed lines = %d, want 1", report.FailedLines)
	}
	if len(store.rows) != 2 {
		t.Fatalf("stored rows = %d, want 2", len(store.rows))
	}
	if store.rows[0].DstCountry != "TestCountry" {
		t.Fatalf("dst country = %q", store.rows[0].DstCountry)
	}
}

func TestImportFileSkipsAlreadyImported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "10.10.10.1_2026-04-28.log-20260429.gz")
	writeSampleGZ(t, path, []string{
		"Apr 28 00:00:23 localhost nat: 日志类型:NAT日志, NAT类型:snat, 源IP:2.55.81.106, 源端口:1799, 目的IP:140.205.70.178, 目的端口:443, 协议:6, 转换后的IP:58.216.48.6, 转换后的端口:1799",
	})

	store := newFakeStore()
	store.imported["10.10.10.1_2026-04-28.log-20260429.gz"] = true
	imp := NewImporter(store, nil, 10)

	report, err := imp.ImportFile(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", report.Skipped)
	}
	if len(store.rows) != 0 {
		t.Fatalf("should not insert any rows, got %d", len(store.rows))
	}
}

func TestImportFileForceReimports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "10.10.10.1_2026-04-28.log-20260429.gz")
	writeSampleGZ(t, path, []string{
		"Apr 28 00:00:23 localhost nat: 日志类型:NAT日志, NAT类型:snat, 源IP:2.55.81.106, 源端口:1799, 目的IP:140.205.70.178, 目的端口:443, 协议:6, 转换后的IP:58.216.48.6, 转换后的端口:1799",
	})

	store := newFakeStore()
	store.imported["10.10.10.1_2026-04-28.log-20260429.gz"] = true
	imp := NewImporter(store, nil, 10)

	report, err := imp.ImportFile(context.Background(), path, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Skipped != 0 {
		t.Fatalf("should not skip, got skipped=%d", report.Skipped)
	}
	if report.Rows != 1 {
		t.Fatalf("rows = %d, want 1", report.Rows)
	}
	if len(store.deleted) != 1 {
		t.Fatalf("should have deleted old rows, got %d", len(store.deleted))
	}
}

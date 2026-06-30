package exporter

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sangfor-log-search/internal/model"
	"sangfor-log-search/internal/query"
)

type fakeJobStore struct {
	jobs map[string]jobRecord
}

type jobRecord struct {
	status     string
	progress   uint64
	total      uint64
	error      string
	resultPath string
	createdAt  time.Time
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{jobs: make(map[string]jobRecord)}
}

func (s *fakeJobStore) Create(ctx context.Context, job interface{}) error {
	return nil
}
func (s *fakeJobStore) Update(ctx context.Context, job interface{}) error {
	return nil
}
func (s *fakeJobStore) Get(ctx context.Context, id string) (interface{}, error) {
	return nil, nil
}
func (s *fakeJobStore) List(ctx context.Context, limit int) ([]interface{}, error) {
	return nil, nil
}

func sampleRow(dstIP, trIP string) model.LogRow {
	return model.LogRow{
		Ts:           time.Date(2026, 4, 28, 0, 0, 23, 0, time.UTC),
		LogDate:      time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC),
		DeviceIP:     parseAddr("10.10.10.1"),
		LogType:      "NAT日志",
		NatType:      "snat",
		SrcIP:        parseAddr("2.55.81.106"),
		SrcPort:      1799,
		DstIP:        parseAddr(dstIP),
		DstPort:      443,
		Protocol:     6,
		TranslatedIP: parseAddr(trIP),
		TranslatedPort: 1799,
		SourceFile:   "10.10.10.1_2026-04-28.log-20260429.gz",
		LineNo:       1,
		DstCountry:   "中国",
	}
}

func parseAddr(s string) netip.Addr {
	return netip.MustParseAddr(s)
}

func fakeQueryFn(rows []model.LogRow) QueryFunc {
	return func(ctx context.Context, f query.LogFilter, limit, offset int) ([]model.LogRow, error) {
		return rows, nil
	}
}

func fakeCountFn(count uint64) CountFunc {
	return func(ctx context.Context, f query.LogFilter) (uint64, error) {
		return count, nil
	}
}

func TestCSVExportWritesHeaderAndRows(t *testing.T) {
	rows := []model.LogRow{
		sampleRow("140.205.70.178", "58.216.48.6"),
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")

	if err := writeCSV(path, rows); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	if !strings.Contains(got, "目的IP") {
		t.Fatalf("csv missing header: %s", got)
	}
	if !strings.Contains(got, "140.205.70.178") {
		t.Fatalf("csv missing dst ip: %s", got)
	}
	if !strings.Contains(got, "58.216.48.6") {
		t.Fatalf("csv missing translated ip: %s", got)
	}
	if !strings.Contains(got, "中国") {
		t.Fatalf("csv missing country: %s", got)
	}
}

func TestLogExportWritesReconstructedLines(t *testing.T) {
	rows := []model.LogRow{
		sampleRow("140.205.70.178", "58.216.48.6"),
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	if err := writeLog(path, rows); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	if !strings.Contains(got, "NAT类型:snat") {
		t.Fatalf("log missing nat type: %s", got)
	}
	if !strings.Contains(got, "140.205.70.178") {
		t.Fatalf("log missing dst ip: %s", got)
	}
	if !strings.Contains(got, "58.216.48.6") {
		t.Fatalf("log missing translated ip: %s", got)
	}
}

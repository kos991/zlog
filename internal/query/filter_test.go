package query

import (
	"strings"
	"testing"
	"time"
)

func mustTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestBuildSelectSQLUsesDstIPAndTimeRange(t *testing.T) {
	f := LogFilter{
		Start:   mustTime("2026-04-28 00:00:00"),
		End:     mustTime("2026-04-28 23:59:59"),
		IP:      "140.205.70.178",
		IPField: "dst",
	}
	sql, args, err := BuildSelectSQL(f, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "dst_ip = ?") {
		t.Fatalf("sql = %s", sql)
	}
	if !strings.Contains(sql, "ts >= ?") {
		t.Fatalf("sql = %s", sql)
	}
	if !strings.Contains(sql, "ts <= ?") {
		t.Fatalf("sql = %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY ts DESC") {
		t.Fatalf("sql = %s", sql)
	}
	if !strings.Contains(sql, "LIMIT 100 OFFSET 0") {
		t.Fatalf("sql = %s", sql)
	}
	if len(args) != 3 {
		t.Fatalf("args = %d, want 3", len(args))
	}
}

func TestBuildSelectSQLAnyIP(t *testing.T) {
	f := LogFilter{
		Start:   mustTime("2026-04-28 00:00:00"),
		End:     mustTime("2026-04-28 23:59:59"),
		IP:      "10.0.0.1",
		IPField: "any",
	}
	sql, args, err := BuildSelectSQL(f, 50, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "src_ip = ? OR dst_ip = ? OR translated_ip = ?") {
		t.Fatalf("sql = %s", sql)
	}
	if len(args) != 5 {
		t.Fatalf("args = %d, want 5", len(args))
	}
	if !strings.Contains(sql, "LIMIT 50 OFFSET 10") {
		t.Fatalf("sql = %s", sql)
	}
}

func TestBuildCountSQL(t *testing.T) {
	f := LogFilter{
		Start: mustTime("2026-04-01 00:00:00"),
		End:   mustTime("2026-04-30 23:59:59"),
	}
	sql, args, err := BuildCountSQL(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "count()") {
		t.Fatalf("sql = %s", sql)
	}
	if !strings.Contains(sql, "ts >= ?") {
		t.Fatalf("sql = %s", sql)
	}
	if len(args) != 2 {
		t.Fatalf("args = %d, want 2", len(args))
	}
}

func TestBuildSelectSQLWithAllFilters(t *testing.T) {
	f := LogFilter{
		Start:    mustTime("2026-04-28 00:00:00"),
		End:      mustTime("2026-04-28 23:59:59"),
		IP:       "1.2.3.4",
		IPField:  "src",
		DeviceIP: "10.10.10.1",
		FilePart: "20260428",
		NatType:  "snat",
		Protocol: "6",
		SrcPort:  1799,
		DstPort:  443,
	}
	sql, _, err := BuildSelectSQL(f, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"src_ip = ?",
		"device_ip = ?",
		"source_file LIKE ?",
		"nat_type = ?",
		"protocol = ?",
		"src_port = ?",
		"dst_port = ?",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("sql missing %q: %s", want, sql)
		}
	}
}

func TestBuildWhereRejectsBadIPField(t *testing.T) {
	f := LogFilter{
		Start:   mustTime("2026-04-28 00:00:00"),
		End:     mustTime("2026-04-28 23:59:59"),
		IP:      "1.2.3.4",
		IPField: "bad",
	}
	_, _, err := BuildSelectSQL(f, 100, 0)
	if err == nil {
		t.Fatal("expected error for bad ip field")
	}
}

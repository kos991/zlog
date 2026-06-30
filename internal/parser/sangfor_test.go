package parser

import (
	"testing"

	"sangfor-log-search/internal/model"
)

func TestParseFilenameAndLine(t *testing.T) {
	meta, err := ParseFilename("10.10.10.1_2026-04-28.log-20260429.gz")
	if err != nil {
		t.Fatal(err)
	}
	if meta.DeviceIP.String() != "10.10.10.1" {
		t.Fatalf("device ip = %s", meta.DeviceIP)
	}
	if meta.SourceFile != "10.10.10.1_2026-04-28.log-20260429.gz" {
		t.Fatalf("source file = %q", meta.SourceFile)
	}

	row, err := ParseLine(meta, "Apr 28 00:00:23 localhost nat: 日志类型:NAT日志, NAT类型:snat, 源IP:2.55.81.106, 源端口:1799, 目的IP:140.205.70.178, 目的端口:443, 协议:6, 转换后的IP:58.216.48.6, 转换后的端口:1799", 1)
	if err != nil {
		t.Fatal(err)
	}
	if row.DstIP.String() != "140.205.70.178" {
		t.Fatalf("dst ip = %s", row.DstIP)
	}
	if row.SrcIP.String() != "2.55.81.106" {
		t.Fatalf("src ip = %s", row.SrcIP)
	}
	if row.TranslatedIP.String() != "58.216.48.6" {
		t.Fatalf("translated ip = %s", row.TranslatedIP)
	}
	if row.SrcPort != 1799 {
		t.Fatalf("src port = %d", row.SrcPort)
	}
	if row.DstPort != 443 {
		t.Fatalf("dst port = %d", row.DstPort)
	}
	if row.Protocol != 6 {
		t.Fatalf("protocol = %d", row.Protocol)
	}
	if row.NatType != "snat" {
		t.Fatalf("nat type = %q", row.NatType)
	}
	if row.LogType != "NAT日志" {
		t.Fatalf("log type = %q", row.LogType)
	}
}

func TestParseFilenameRejectsBadFormat(t *testing.T) {
	_, err := ParseFilename("random-file.log")
	if err == nil {
		t.Fatal("expected error for bad filename")
	}
}

func TestParseLineRejectsMissingFields(t *testing.T) {
	meta, _ := ParseFilename("10.10.10.1_2026-04-28.log-20260429.gz")
	_, err := ParseLine(meta, "Apr 28 00:00:23 localhost nat: 日志类型:NAT日志", 1)
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestParseLinesCountsErrorsWithoutStopping(t *testing.T) {
	meta, _ := ParseFilename("10.10.10.1_2026-04-28.log-20260429.gz")
	lines := []string{
		"",
		"Apr 28 00:00:23 localhost nat: 日志类型:NAT日志, NAT类型:snat, 源IP:2.55.81.106, 源端口:1799, 目的IP:140.205.70.178, 目的端口:443, 协议:6, 转换后的IP:58.216.48.6, 转换后的端口:1799",
		"bad line without nat prefix",
		"Apr 28 00:00:24 localhost nat: 日志类型:NAT日志, NAT类型:dnat, 源IP:1.1.1.1, 源端口:80, 目的IP:2.2.2.2, 目的端口:443, 协议:6, 转换后的IP:3.3.3.3, 转换后的端口:80",
	}

	rows, errs := ParseLines(meta, lines)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %d, want 1", len(errs))
	}
	if errs[0].LineNo != 3 {
		t.Fatalf("error line no = %d, want 3", errs[0].LineNo)
	}
}

func TestParseFilenameMultipleDevices(t *testing.T) {
	cases := []struct {
		filename string
		ip       string
	}{
		{"10.10.10.1_2026-04-23.log-20260427.gz", "10.10.10.1"},
		{"192.168.9.13_2026-04-24.log-20260428.gz", "192.168.9.13"},
	}
	for _, tc := range cases {
		meta, err := ParseFilename(tc.filename)
		if err != nil {
			t.Fatalf("%s: %v", tc.filename, err)
		}
		if meta.DeviceIP.String() != tc.ip {
			t.Fatalf("%s: device ip = %s, want %s", tc.filename, meta.DeviceIP, tc.ip)
		}
	}
}

var _ model.FileMeta

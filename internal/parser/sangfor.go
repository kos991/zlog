package parser

import (
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sangfor-log-search/internal/model"
)

var (
	filenamePattern = regexp.MustCompile(`^(\d+\.\d+\.\d+\.\d+)_(\d{4}-\d{2}-\d{2})\.log-(\d{8})\.gz$`)
)

func ParseFilename(name string) (model.FileMeta, error) {
	matches := filenamePattern.FindStringSubmatch(name)
	if matches == nil {
		return model.FileMeta{}, fmt.Errorf("unrecognized filename format: %s", name)
	}

	ip, err := netip.ParseAddr(matches[1])
	if err != nil {
		return model.FileMeta{}, fmt.Errorf("parse device ip: %w", err)
	}

	logDate, err := time.Parse("2006-01-02", matches[2])
	if err != nil {
		return model.FileMeta{}, fmt.Errorf("parse log date: %w", err)
	}

	archiveDate, err := time.ParseInLocation("20060102", matches[3], time.Local)
	if err != nil {
		return model.FileMeta{}, fmt.Errorf("parse archive date: %w", err)
	}

	return model.FileMeta{
		SourceFile:  name,
		DeviceIP:    ip,
		LogDate:     logDate,
		ArchiveDate: archiveDate,
	}, nil
}

func parseLineField(line string, field string) (string, bool) {
	key := field + ":"
	idx := strings.Index(line, key)
	if idx < 0 {
		return "", false
	}
	start := idx + len(key)
	rest := line[start:]
	commaIdx := strings.Index(rest, ",")
	if commaIdx < 0 {
		return strings.TrimSpace(rest), true
	}
	return strings.TrimSpace(rest[:commaIdx]), true
}

func ParseLine(meta model.FileMeta, line string, lineNo uint32) (model.LogRow, error) {
	prefix := "nat: "
	natIdx := strings.Index(line, prefix)
	if natIdx < 0 {
		return model.LogRow{}, fmt.Errorf("no nat: prefix in line %d", lineNo)
	}
	body := line[natIdx+len(prefix):]

	logType, ok := parseLineField(body, "日志类型")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing 日志类型 in line %d", lineNo)
	}
	logType = strings.TrimSpace(logType)

	natType, ok := parseLineField(body, "NAT类型")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing NAT类型 in line %d", lineNo)
	}
	natType = strings.TrimSpace(natType)

	srcIPStr, ok := parseLineField(body, "源IP")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing 源IP in line %d", lineNo)
	}
	srcIP, err := netip.ParseAddr(strings.TrimSpace(srcIPStr))
	if err != nil {
		return model.LogRow{}, fmt.Errorf("parse 源IP: %w", err)
	}

	srcPortStr, ok := parseLineField(body, "源端口")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing 源端口 in line %d", lineNo)
	}
	srcPort, err := strconv.ParseUint(strings.TrimSpace(srcPortStr), 10, 16)
	if err != nil {
		return model.LogRow{}, fmt.Errorf("parse 源端口: %w", err)
	}

	dstIPStr, ok := parseLineField(body, "目的IP")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing 目的IP in line %d", lineNo)
	}
	dstIP, err := netip.ParseAddr(strings.TrimSpace(dstIPStr))
	if err != nil {
		return model.LogRow{}, fmt.Errorf("parse 目的IP: %w", err)
	}

	dstPortStr, ok := parseLineField(body, "目的端口")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing 目的端口 in line %d", lineNo)
	}
	dstPort, err := strconv.ParseUint(strings.TrimSpace(dstPortStr), 10, 16)
	if err != nil {
		return model.LogRow{}, fmt.Errorf("parse 目的端口: %w", err)
	}

	protoStr, ok := parseLineField(body, "协议")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing 协议 in line %d", lineNo)
	}
	protocol, err := strconv.ParseUint(strings.TrimSpace(protoStr), 10, 8)
	if err != nil {
		return model.LogRow{}, fmt.Errorf("parse 协议: %w", err)
	}

	trIPStr, ok := parseLineField(body, "转换后的IP")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing 转换后的IP in line %d", lineNo)
	}
	trIP, err := netip.ParseAddr(strings.TrimSpace(trIPStr))
	if err != nil {
		return model.LogRow{}, fmt.Errorf("parse 转换后的IP: %w", err)
	}

	trPortStr, ok := parseLineField(body, "转换后的端口")
	if !ok {
		return model.LogRow{}, fmt.Errorf("missing 转换后的端口 in line %d", lineNo)
	}
	trPort, err := strconv.ParseUint(strings.TrimSpace(trPortStr), 10, 16)
	if err != nil {
		return model.LogRow{}, fmt.Errorf("parse 转换后的端口: %w", err)
	}

	tsStr := strings.TrimSpace(line[:natIdx])
	ts, err := parseTimestamp(tsStr, meta.LogDate)
	if err != nil {
		return model.LogRow{}, fmt.Errorf("parse timestamp: %w", err)
	}

	return model.LogRow{
		Ts:             ts,
		LogDate:        meta.LogDate,
		DeviceIP:       meta.DeviceIP,
		LogType:        logType,
		NatType:        natType,
		SrcIP:          srcIP,
		SrcPort:        uint16(srcPort),
		DstIP:          dstIP,
		DstPort:        uint16(dstPort),
		Protocol:       uint8(protocol),
		TranslatedIP:   trIP,
		TranslatedPort: uint16(trPort),
		SourceFile:     meta.SourceFile,
		LineNo:         lineNo,
	}, nil
}

func parseTimestamp(tsStr string, logDate time.Time) (time.Time, error) {
	tsStr = strings.TrimSpace(tsStr)
	if tsStr == "" {
		return logDate, nil
	}
	for _, layout := range []string{"Jan 2 15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, tsStr, time.Local); err == nil {
			return t.AddDate(logDate.Year(), 0, 0), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp: %s", tsStr)
}

func ParseLines(meta model.FileMeta, lines []string) ([]model.LogRow, []model.ParseError) {
	rows := make([]model.LogRow, 0, len(lines))
	var errs []model.ParseError
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lineNo := uint32(i + 1)
		row, err := ParseLine(meta, line, lineNo)
		if err != nil {
			errs = append(errs, model.ParseError{
				SourceFile: meta.SourceFile,
				LineNo:     lineNo,
				Line:       line,
				Err:        err,
			})
			continue
		}
		rows = append(rows, row)
	}
	return rows, errs
}

package query

import (
	"fmt"
	"strings"
	"time"
)

type LogFilter struct {
	Start, End time.Time
	IP         string
	IPField    string
	DeviceIP   string
	FilePart   string
	NatType    string
	Protocol   string
	SrcPort    uint16
	DstPort    uint16
	TrPort     uint16
	Keyword    string
}

func (f LogFilter) Validate() error {
	if f.Start.IsZero() || f.End.IsZero() {
		return fmt.Errorf("time range is required")
	}
	if f.Start.After(f.End) {
		return fmt.Errorf("start must be before end")
	}
	return nil
}

func ipColumn(field string) (string, error) {
	switch field {
	case "", "dst":
		return "dst_ip", nil
	case "src":
		return "src_ip", nil
	case "tr", "translated":
		return "translated_ip", nil
	case "any", "all":
		return "", nil
	default:
		return "", fmt.Errorf("unknown ip field: %s", field)
	}
}

func BuildWhere(f LogFilter) (string, []any, error) {
	var parts []string
	var args []any

	if !f.Start.IsZero() {
		parts = append(parts, "ts >= ?")
		args = append(args, f.Start)
	}
	if !f.End.IsZero() {
		parts = append(parts, "ts <= ?")
		args = append(args, f.End)
	}

	if f.IP != "" {
		col, err := ipColumn(f.IPField)
		if err != nil {
			return "", nil, err
		}
		if col != "" {
			parts = append(parts, fmt.Sprintf("%s = ?", col))
			args = append(args, f.IP)
		} else {
			parts = append(parts, "(src_ip = ? OR dst_ip = ? OR translated_ip = ?)")
			args = append(args, f.IP, f.IP, f.IP)
		}
	}

	if f.DeviceIP != "" {
		parts = append(parts, "device_ip = ?")
		args = append(args, f.DeviceIP)
	}
	if f.FilePart != "" {
		parts = append(parts, "source_file LIKE ?")
		args = append(args, "%"+f.FilePart+"%")
	}
	if f.NatType != "" {
		parts = append(parts, "nat_type = ?")
		args = append(args, f.NatType)
	}
	if f.Protocol != "" {
		parts = append(parts, "protocol = ?")
		args = append(args, f.Protocol)
	}
	if f.SrcPort != 0 {
		parts = append(parts, "src_port = ?")
		args = append(args, f.SrcPort)
	}
	if f.DstPort != 0 {
		parts = append(parts, "dst_port = ?")
		args = append(args, f.DstPort)
	}
	if f.TrPort != 0 {
		parts = append(parts, "translated_port = ?")
		args = append(args, f.TrPort)
	}

	where := ""
	if len(parts) > 0 {
		where = "WHERE " + strings.Join(parts, " AND ")
	}
	return where, args, nil
}

func BuildSelectSQL(f LogFilter, limit, offset int) (string, []any, error) {
	where, args, err := BuildWhere(f)
	if err != nil {
		return "", nil, err
	}

	sql := fmt.Sprintf(
		"SELECT ts, log_date, device_ip, src_ip, src_port, dst_ip, dst_port, protocol, translated_ip, translated_port, log_type, nat_type, dst_country, source_file, line_no, imported_at FROM nat_logs %s ORDER BY ts DESC LIMIT %d OFFSET %d",
		where, limit, offset,
	)
	return sql, args, nil
}

func BuildCountSQL(f LogFilter) (string, []any, error) {
	where, args, err := BuildWhere(f)
	if err != nil {
		return "", nil, err
	}
	sql := fmt.Sprintf("SELECT count() FROM nat_logs %s", where)
	return sql, args, nil
}

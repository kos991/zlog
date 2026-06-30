package exporter

import (
	"encoding/csv"
	"fmt"
	"os"

	"sangfor-log-search/internal/model"
)

var csvHeader = []string{
	"时间", "日志日期", "设备IP", "日志类型", "NAT类型",
	"源IP", "源端口", "目的IP", "目的IP归属", "目的端口",
	"协议", "转换后IP", "转换后端口", "来源文件", "行号",
}

func rowToCSVRecord(r model.LogRow) []string {
	return []string{
		r.Ts.Format("2006-01-02 15:04:05"),
		r.LogDate.Format("2006-01-02"),
		r.DeviceIP.String(),
		r.LogType,
		r.NatType,
		r.SrcIP.String(),
		fmt.Sprintf("%d", r.SrcPort),
		r.DstIP.String(),
		r.DstCountry,
		fmt.Sprintf("%d", r.DstPort),
		fmt.Sprintf("%d", r.Protocol),
		r.TranslatedIP.String(),
		fmt.Sprintf("%d", r.TranslatedPort),
		r.SourceFile,
		fmt.Sprintf("%d", r.LineNo),
	}
}

func writeCSV(path string, rows []model.LogRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(csvHeader); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write(rowToCSVRecord(r)); err != nil {
			return err
		}
	}
	return nil
}

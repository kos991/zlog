package exporter

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"sangfor-log-search/internal/model"
)

func rowToLogLine(r model.LogRow) string {
	return fmt.Sprintf("%s localhost nat: 日志类型:%s, NAT类型:%s, 源IP:%s, 源端口:%d, 目的IP:%s, 目的端口:%d, 协议:%d, 转换后的IP:%s, 转换后的端口:%d",
		r.Ts.Format("Jan 2 15:04:05"),
		r.LogType,
		r.NatType,
		r.SrcIP.String(),
		r.SrcPort,
		r.DstIP.String(),
		r.DstPort,
		r.Protocol,
		r.TranslatedIP.String(),
		r.TranslatedPort,
	)
}

func writeLog(path string, rows []model.LogRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	for _, r := range rows {
		line := strings.TrimSpace(rowToLogLine(r))
		if _, err := w.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	return nil
}

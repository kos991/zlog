//go:build cgo

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"sangfor-log-search/internal/logsearch"
)

const (
	defaultLogDir     = "/data/sangfor_fw_log"
	defaultDBPath     = "/opt/sangfor-log-search/db/sangfor_logs"
	defaultExportPath = "/data/query_result.log"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	if err := logsearch.InitializeZvec(); err != nil {
		fatalf("初始化 zvec 失败：%v", err)
	}
	defer func() {
		_ = logsearch.ShutdownZvec()
	}()

	switch os.Args[1] {
	case "import":
		runImport(os.Args[2:])
	case "query":
		runQuery(os.Args[2:])
	case "version":
		fmt.Println("sangfor-log-search 0.1.0")
	default:
		usage()
		os.Exit(2)
	}
}

func runImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	logDir := fs.String("log-dir", envOrDefault("LOG_SEARCH_LOG_DIR", defaultLogDir), "日志目录")
	dbPath := fs.String("db", envOrDefault("LOG_SEARCH_DB", defaultDBPath), "zvec 数据库目录")
	batchSize := fs.Int("batch", 1000, "批量写入行数")
	_ = fs.Parse(args)

	store, err := logsearch.CreateOrOpenStore(*dbPath)
	if err != nil {
		fatalf("打开数据库失败：%v", err)
	}
	defer store.Close()

	total, err := store.ImportDir(*logDir, *batchSize, os.Stdout)
	if err != nil {
		fatalf("导入失败：%v", err)
	}
	fmt.Printf("导入完成，共 %d 行，数据库：%s\n", total, *dbPath)
}

func runQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	dbPath := fs.String("db", envOrDefault("LOG_SEARCH_DB", defaultDBPath), "zvec 数据库目录")
	ip := fs.String("ip", "", "按 IP 精确查询")
	keyword := fs.String("keyword", "", "按关键词查询")
	start := fs.String("start", "", "开始日期：YYYY / YYYYMM / YYYYMMDD")
	end := fs.String("end", "", "结束日期：YYYY / YYYYMM / YYYYMMDD")
	source := fs.String("file", "", "按文件名片段过滤")
	limit := fs.Int("limit", 200, "最大返回条数")
	export := fs.String("export", "", "导出结果路径")
	_ = fs.Parse(args)

	startKey, err := logsearch.NormalizeDateKey(*start)
	if err != nil {
		fatalf("%v", err)
	}
	endKey, err := logsearch.NormalizeEndDateKey(*end)
	if err != nil {
		fatalf("%v", err)
	}

	store, err := logsearch.CreateOrOpenStore(*dbPath)
	if err != nil {
		fatalf("打开数据库失败：%v", err)
	}
	defer store.Close()

	records, err := store.Query(logsearch.QueryOptions{
		IP:         *ip,
		Keyword:    *keyword,
		StartDate:  startKey,
		EndDate:    endKey,
		SourceLike: *source,
		Limit:      *limit,
	})
	if err != nil {
		fatalf("查询失败：%v", err)
	}

	if err := logsearch.WriteRecords(records, os.Stdout); err != nil {
		fatalf("输出失败：%v", err)
	}
	fmt.Fprintf(os.Stderr, "查询完成，共 %d 条\n", len(records))

	if *export != "" {
		if err := os.MkdirAll(filepath.Dir(*export), 0755); err != nil {
			fatalf("创建导出目录失败：%v", err)
		}
		file, err := os.Create(*export)
		if err != nil {
			fatalf("创建导出文件失败：%v", err)
		}
		if err := logsearch.WriteRecords(records, file); err != nil {
			_ = file.Close()
			fatalf("写入导出文件失败：%v", err)
		}
		if err := file.Close(); err != nil {
			fatalf("关闭导出文件失败：%v", err)
		}
		fmt.Fprintf(os.Stderr, "已导出：%s\n", *export)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `用法：
  log-search import [--log-dir /data/sangfor_fw_log] [--db /opt/sangfor-log-search/db/sangfor_logs]
  log-search query --ip 10.10.10.1 [--start 20260401 --end 20260430] [--keyword deny] [--file 20260429] [--limit 200]
  log-search query --keyword deny --export /data/query_result.log
  log-search version`)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}


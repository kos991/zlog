package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"sangfor-log-search/internal/model"
	"sangfor-log-search/internal/query"
)

type ClickHouseStore struct {
	conn driver.Conn
}

func Open(ctx context.Context, url string) (*ClickHouseStore, error) {
	opts, err := clickhouse.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse url: %w", err)
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}
	return &ClickHouseStore{conn: conn}, nil
}

func (s *ClickHouseStore) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *ClickHouseStore) Conn() driver.Conn {
	return s.conn
}

func (s *ClickHouseStore) EnsureSchema(ctx context.Context) error {
	ddl := []string{
		`CREATE DATABASE IF NOT EXISTS zlog`,
		`USE zlog`,
		`CREATE TABLE IF NOT EXISTS nat_logs
		(
			ts              DateTime CODEC(Delta, ZSTD(3)),
			log_date        Date CODEC(Delta, ZSTD(3)),
			device_ip       IPv4 CODEC(ZSTD(3)),
			src_ip          IPv4 CODEC(ZSTD(3)),
			src_port        UInt16 CODEC(T64, ZSTD(3)),
			dst_ip          IPv4 CODEC(ZSTD(3)),
			dst_port        UInt16 CODEC(T64, ZSTD(3)),
			protocol        UInt8 CODEC(ZSTD(3)),
			translated_ip   IPv4 CODEC(ZSTD(3)),
			translated_port UInt16 CODEC(T64, ZSTD(3)),
			log_type        LowCardinality(String) CODEC(ZSTD(3)),
			nat_type        LowCardinality(String) CODEC(ZSTD(3)),
			dst_country     LowCardinality(String) CODEC(ZSTD(3)),
			source_file     LowCardinality(String) CODEC(ZSTD(3)),
			line_no         UInt32 CODEC(Delta, ZSTD(3)),
			imported_at     DateTime CODEC(Delta, ZSTD(3))
		)
		ENGINE = MergeTree
		PARTITION BY toYYYYMM(log_date)
		ORDER BY (log_date, dst_ip, ts)`,
		`CREATE TABLE IF NOT EXISTS import_state
		(
			source_file  String CODEC(ZSTD(3)),
			file_hash    String CODEC(ZSTD(3)),
			rows         UInt64 CODEC(T64, ZSTD(3)),
			failed_lines UInt64 CODEC(T64, ZSTD(3)),
			imported_at  DateTime CODEC(Delta, ZSTD(3)),
			status       LowCardinality(String) CODEC(ZSTD(3))
		)
		ENGINE = ReplacingMergeTree
		ORDER BY (source_file)`,
		`CREATE TABLE IF NOT EXISTS jobs
		(
			id          String CODEC(ZSTD(3)),
			type        LowCardinality(String) CODEC(ZSTD(3)),
			status      LowCardinality(String) CODEC(ZSTD(3)),
			source      String CODEC(ZSTD(3)),
			progress    UInt64 CODEC(T64, ZSTD(3)),
			total       UInt64 CODEC(T64, ZSTD(3)),
			error       String CODEC(ZSTD(3)),
			result_path String CODEC(ZSTD(3)),
			created_at  DateTime CODEC(Delta, ZSTD(3)),
			updated_at  DateTime CODEC(Delta, ZSTD(3))
		)
		ENGINE = ReplacingMergeTree
		ORDER BY (created_at, id)`,
	}
	for _, stmt := range ddl {
		if err := s.conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("exec ddl: %w", err)
		}
	}
	return nil
}

func (s *ClickHouseStore) InsertBatch(ctx context.Context, rows []model.LogRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO zlog.nat_logs (ts, log_date, device_ip, src_ip, src_port, dst_ip, dst_port, protocol, translated_ip, translated_port, log_type, nat_type, dst_country, source_file, line_no, imported_at)")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	now := time.Now()
	for _, r := range rows {
		country := r.DstCountry
		if country == "" {
			country = "未知"
		}
		importedAt := r.ImportedAt
		if importedAt.IsZero() {
			importedAt = now
		}
		if err := batch.Append(
			r.Ts,
			r.LogDate,
			r.DeviceIP.String(),
			r.SrcIP.String(),
			r.SrcPort,
			r.DstIP.String(),
			r.DstPort,
			r.Protocol,
			r.TranslatedIP.String(),
			r.TranslatedPort,
			r.LogType,
			r.NatType,
			country,
			r.SourceFile,
			r.LineNo,
			importedAt,
		); err != nil {
			return fmt.Errorf("append row: %w", err)
		}
	}
	return batch.Send()
}

func (s *ClickHouseStore) Query(ctx context.Context, f query.LogFilter, limit, offset int) ([]model.LogRow, error) {
	sql, args, err := query.BuildSelectSQL(f, limit, offset)
	if err != nil {
		return nil, err
	}
	rows, err := s.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var result []model.LogRow
	for rows.Next() {
		var r model.LogRow
		var deviceIP, srcIP, dstIP, trIP string
		var importedAt time.Time
		if err := rows.Scan(
			&r.Ts, &r.LogDate, &deviceIP, &srcIP, &r.SrcPort, &dstIP, &r.DstPort, &r.Protocol,
			&trIP, &r.TranslatedPort, &r.LogType, &r.NatType, &r.DstCountry,
			&r.SourceFile, &r.LineNo, &importedAt,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		r.DeviceIP = parseIP(deviceIP)
		r.SrcIP = parseIP(srcIP)
		r.DstIP = parseIP(dstIP)
		r.TranslatedIP = parseIP(trIP)
		r.ImportedAt = importedAt
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *ClickHouseStore) Count(ctx context.Context, f query.LogFilter) (uint64, error) {
	sql, args, err := query.BuildCountSQL(f)
	if err != nil {
		return 0, err
	}
	var count uint64
	if err := s.conn.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return count, nil
}

func (s *ClickHouseStore) RecordImportState(ctx context.Context, sourceFile, fileHash string, rows, failedLines uint64, status string) error {
	return s.conn.Exec(ctx,
		"INSERT INTO zlog.import_state (source_file, file_hash, rows, failed_lines, imported_at, status) VALUES (?, ?, ?, ?, ?, ?)",
		sourceFile, fileHash, rows, failedLines, time.Now(), status,
	)
}

func (s *ClickHouseStore) IsFileImported(ctx context.Context, sourceFile string) (bool, error) {
	var count uint64
	err := s.conn.QueryRow(ctx,
		"SELECT count() FROM zlog.import_state WHERE source_file = ? AND status = 'ok'",
		sourceFile,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *ClickHouseStore) DeleteFileRows(ctx context.Context, sourceFile string) error {
	return s.conn.Exec(ctx, "ALTER TABLE zlog.nat_logs DELETE WHERE source_file = ?", sourceFile)
}

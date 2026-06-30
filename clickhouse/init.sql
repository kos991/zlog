-- zlog ClickHouse schema
-- Table: nat_logs

CREATE TABLE IF NOT EXISTS nat_logs
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
ORDER BY (log_date, dst_ip, ts);

-- Table: import_state (file-level dedupe)

CREATE TABLE IF NOT EXISTS import_state
(
    source_file  String CODEC(ZSTD(3)),
    file_hash    String CODEC(ZSTD(3)),
    rows         UInt64 CODEC(T64, ZSTD(3)),
    failed_lines UInt64 CODEC(T64, ZSTD(3)),
    imported_at  DateTime CODEC(Delta, ZSTD(3)),
    status       LowCardinality(String) CODEC(ZSTD(3))
)
ENGINE = ReplacingMergeTree
ORDER BY (source_file);

-- Table: jobs (async export/import job state)

CREATE TABLE IF NOT EXISTS jobs
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
ORDER BY (created_at, id);

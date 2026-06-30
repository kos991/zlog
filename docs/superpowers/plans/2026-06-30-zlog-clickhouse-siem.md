# zlog ClickHouse SIEM Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Docker Compose-deployed zlog service backed by ClickHouse for fast NAT log search by time and IP, with destination IP geo labeling and async export.

**Architecture:** A Go web service parses Sangfor firewall NAT logs and writes only structured columns into ClickHouse. The UI and APIs stay in the same service; ClickHouse is the only long-lived query store. Docker Compose runs both containers, and the import path is one-way ingestion so source files can be removed after successful import.

**Tech Stack:** Go 1.22, ClickHouse 24.x, Docker Compose, `clickhouse-go/v2`, `html/template`, vanilla HTML/CSS/JS, `bcrypt`, `jq`.

## Global Constraints

- ClickHouse is the only main database.
- Source files are ingest input only and may be deleted after successful import.
- Do not store `raw_line`.
- All query and export paths must use ClickHouse.
- Default table layout is `PARTITION BY toYYYYMM(log_date)` and `ORDER BY (log_date, dst_ip, ts)`.
- Column compression defaults to `Delta`, `T64`, and `ZSTD(3)`.
- Destination IP geo label must be stored in `dst_country`.
- Custom CIDR overrides builtin geo labels.
- Query defaults: time range required, page size 100, max range 365 days.
- No build artifacts committed to the repository.

---

### Task 1: Bootstrap the service, config loader, and Docker runtime

**Files:**
- Create: `go.mod`
- Create: `cmd/zlog/main.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`
- Create: `clickhouse/init.sql`
- Create: `docker-compose.yml`
- Create: `README.md`

**Interfaces:**
- Consumes: a YAML config path from `main`.
- Produces:
  - `type Config struct` with `Server`, `Auth`, `Paths`, `Import`, `Query`, `Storage`, and `Export` sections.
  - `func Load(path string) (Config, error)`.
  - `type App struct` with `Run(ctx context.Context) error`.
  - `func New(cfg Config) (*App, error)`.

- [ ] **Step 1: Write the failing test**

```go
// internal/config/config_test.go
func TestLoadMergesDefaultsAndFile(t *testing.T) {
    cfg, err := Load("testdata/config.yaml")
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Query.DefaultPageSize != 100 {
        t.Fatalf("default page size = %d", cfg.Query.DefaultPageSize)
    }
    if cfg.Storage.OrderBy != "log_date,dst_ip,ts" {
        t.Fatalf("order by = %q", cfg.Storage.OrderBy)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/config -run TestLoadMergesDefaultsAndFile -v
```

Expected: fail because `Load` is not implemented.

- [ ] **Step 3: Write the minimal implementation**

```go
// internal/config/config.go
type Config struct {
    Server struct {
        Listen        string `yaml:"listen"`
        SessionSecret string `yaml:"session_secret"`
    } `yaml:"server"`
    Auth struct {
        AdminUsername     string `yaml:"admin_username"`
        AdminPasswordHash string `yaml:"admin_password_hash"`
    } `yaml:"auth"`
    Paths struct {
        ClickHouseURL string `yaml:"clickhouse_url"`
        ExportDir     string `yaml:"export_dir"`
        AppLog        string `yaml:"app_log"`
    } `yaml:"paths"`
    Import struct {
        ScanIntervalSeconds int `yaml:"scan_interval_seconds"`
        BatchSize           int `yaml:"batch_size"`
    } `yaml:"import"`
    Query struct {
        RequireTimeRange bool `yaml:"require_time_range"`
        MaxRangeDays     int  `yaml:"max_range_days"`
        DefaultPageSize  int  `yaml:"default_page_size"`
        MaxPageSize      int  `yaml:"max_page_size"`
    } `yaml:"query"`
    Storage struct {
        Compression          string `yaml:"compression"`
        PartitionGranularity  string `yaml:"partition_granularity"`
        OrderBy               string `yaml:"order_by"`
    } `yaml:"storage"`
    Export struct {
        MaxRows       int `yaml:"max_rows"`
        RetentionDays int `yaml:"retention_days"`
    } `yaml:"export"`
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:

```bash
go test ./internal/config ./internal/app ./...
docker compose config
```

Expected: tests pass, Compose config renders.

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/zlog/main.go internal/config internal/app clickhouse/init.sql docker-compose.yml README.md
git commit -m "feat: bootstrap zlog runtime"
```

---

### Task 2: Parse Sangfor NAT filenames and log lines

**Files:**
- Create: `internal/model/log.go`
- Create: `internal/parser/filename.go`
- Create: `internal/parser/sangfor.go`
- Create: `internal/parser/filename_test.go`
- Create: `internal/parser/sangfor_test.go`
- Create: `internal/parser/testdata/nat-sample.log`

**Interfaces:**
- Consumes: `model.FileMeta`.
- Produces:
  - `type FileMeta struct`.
  - `type LogRow struct`.
  - `func ParseFilename(name string) (FileMeta, error)`.
  - `func ParseLine(meta FileMeta, line string, lineNo uint32) (LogRow, error)`.
  - `func ParseLines(meta FileMeta, lines []string) ([]LogRow, []ParseError)`.

- [ ] **Step 1: Write the failing test**

```go
func TestParseFilenameAndLine(t *testing.T) {
    meta, err := ParseFilename("10.10.10.1_2026-04-28.log-20260429.gz")
    if err != nil {
        t.Fatal(err)
    }
    if meta.DeviceIP.String() != "10.10.10.1" {
        t.Fatalf("device ip = %s", meta.DeviceIP)
    }

    row, err := ParseLine(meta, "Apr 28 00:00:23 localhost nat: 日志类型:NAT日志, NAT类型:snat, 源IP:2.55.81.106, 源端口:1799, 目的IP:140.205.70.178, 目的端口:443, 协议:6, 转换后的IP:58.216.48.6, 转换后的端口:1799", 1)
    if err != nil {
        t.Fatal(err)
    }
    if row.DstIP.String() != "140.205.70.178" {
        t.Fatalf("dst ip = %s", row.DstIP)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/parser -run TestParseFilenameAndLine -v
```

Expected: fail because parser functions do not exist yet.

- [ ] **Step 3: Write the minimal implementation**

```go
type FileMeta struct {
    SourceFile  string
    DeviceIP    netip.Addr
    LogDate     time.Time
    ArchiveDate time.Time
}

type LogRow struct {
    Ts             time.Time
    DeviceIP       netip.Addr
    LogType        string
    NatType        string
    SrcIP          netip.Addr
    SrcPort        uint16
    DstIP          netip.Addr
    DstPort        uint16
    Protocol       uint8
    TranslatedIP   netip.Addr
    TranslatedPort uint16
    SourceFile     string
    LineNo         uint32
    ImportedAt     time.Time
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:

```bash
go test ./internal/parser ./...
```

Expected: filename and line parsing tests pass, malformed names and lines are rejected.

- [ ] **Step 5: Commit**

```bash
git add internal/model internal/parser
git commit -m "feat: parse sangfor nat logs"
```

---

### Task 3: Build the ClickHouse store and SQL query builder

**Files:**
- Create: `internal/query/filter.go`
- Create: `internal/query/sql.go`
- Create: `internal/query/sql_test.go`
- Create: `internal/store/clickhouse.go`
- Create: `internal/store/clickhouse_test.go`
- Modify: `clickhouse/init.sql`

**Interfaces:**
- Consumes:
  - `model.LogRow`.
  - `query.LogFilter`.
- Produces:
  - `func BuildSelectSQL(f LogFilter, limit, offset int) (string, []any, error)`.
  - `func BuildCountSQL(f LogFilter) (string, []any, error)`.
  - `type ClickHouseStore struct`.
  - `func (s *ClickHouseStore) InsertBatch(ctx context.Context, rows []model.LogRow) error`.
  - `func (s *ClickHouseStore) Query(ctx context.Context, f query.LogFilter, limit, offset int) ([]model.LogRow, error)`.

- [ ] **Step 1: Write the failing test**

```go
func TestBuildSelectSQLUsesDstIPAndTimeRange(t *testing.T) {
    f := LogFilter{
        Start:   mustTime("2026-04-28 00:00:00"),
        End:     mustTime("2026-04-28 23:59:59"),
        IP:      "140.205.70.178",
        IPField: "dst",
        Page:    1,
    }
    sql, args, err := BuildSelectSQL(f, 100, 0)
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(sql, "dst_ip = ?") {
        t.Fatalf("sql = %s", sql)
    }
    if len(args) != 3 {
        t.Fatalf("args = %d", len(args))
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/query -run TestBuildSelectSQLUsesDstIPAndTimeRange -v
```

Expected: fail because the query builder is not implemented.

- [ ] **Step 3: Write the minimal implementation**

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:

```bash
go test ./internal/query ./internal/store -v
```

Expected: SQL builder tests pass and store unit tests can open a test database.

- [ ] **Step 5: Commit**

```bash
git add internal/query internal/store clickhouse/init.sql
git commit -m "feat: add clickhouse query layer"
```

---

### Task 4: Implement the import pipeline and file-level dedupe

**Files:**
- Create: `internal/importer/importer.go`
- Create: `internal/importer/reader.go`
- Create: `internal/importer/state.go`
- Create: `internal/importer/importer_test.go`
- Create: `internal/importer/testdata/nat-sample.gz`
- Create: `internal/importer/testdata/bad-line.gz`

**Interfaces:**
- Consumes:
  - `parser.ParseFilename`.
  - `parser.ParseLines`.
  - `store.InsertBatch`.
  - `state.Store`.
- Produces:
  - `type ImportReport struct`.
  - `type FileState struct`.
  - `func (i *Importer) ImportFile(ctx context.Context, path string, force bool) (ImportReport, error)`.
  - `func (i *Importer) ImportDirectory(ctx context.Context, dir string) ([]ImportReport, error)`.

- [ ] **Step 1: Write the failing test**

```go
func TestImportFileCountsBadLinesWithoutStopping(t *testing.T) {
    imp := NewImporter(fakeStore, fakeState, fakeResolver)
    report, err := imp.ImportFile(context.Background(), "testdata/bad-line.gz", false)
    if err != nil {
        t.Fatal(err)
    }
    if report.Rows == 0 || report.FailedLines == 0 {
        t.Fatalf("report = %+v", report)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/importer -run TestImportFileCountsBadLinesWithoutStopping -v
```

Expected: fail because importer and state store are not implemented.

- [ ] **Step 3: Write the minimal implementation**

```go
type ImportReport struct {
    Files       int
    Rows        int
    FailedLines int
    Skipped     int
    StartedAt   time.Time
    FinishedAt  time.Time
    Error       string
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:

```bash
go test ./internal/importer ./... -v
```

Expected: gzip fixtures import, duplicate file state is skipped, forced reimport replaces old rows.

- [ ] **Step 5: Commit**

```bash
git add internal/importer
git commit -m "feat: add nat import pipeline"
```

---

### Task 5: Add destination IP geo resolution and custom CIDR overrides

**Files:**
- Create: `internal/ipmeta/resolve.go`
- Create: `internal/ipmeta/loader.go`
- Create: `internal/ipmeta/resolve_test.go`
- Create: `internal/ipmeta/testdata/ipmeta-small.json`
- Create: `internal/ipmeta/testdata/custom-ranges.yaml`
- Create: `scripts/update-ipmeta.sh`

**Interfaces:**
- Consumes: builtin snapshot data and admin-maintained custom CIDR config.
- Produces:
  - `type Region struct`.
  - `type Resolver interface { Resolve(ip netip.Addr) (Region, bool) }`.
  - `func LoadBuiltin(path string) (*Catalog, error)`.
  - `func LoadCustom(path string) (*Catalog, error)`.
  - `func (c *Catalog) Resolve(ip netip.Addr) (Region, bool)`.

- [ ] **Step 1: Write the failing test**

```go
func TestCustomCIDROverridesBuiltinGeo(t *testing.T) {
    builtin := mustCatalog(t, "testdata/ipmeta-small.json")
    custom := mustCustom(t, "testdata/custom-ranges.yaml")
    resolver := NewResolver(builtin, custom)

    region, ok := resolver.Resolve(netip.MustParseAddr("58.216.48.6"))
    if !ok {
        t.Fatal("no region")
    }
    if region.DisplayName != "出口公网IP" {
        t.Fatalf("display = %q", region.DisplayName)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/ipmeta -run TestCustomCIDROverridesBuiltinGeo -v
```

Expected: fail because the resolver is not implemented.

- [ ] **Step 3: Write the minimal implementation**

```go
type Region struct {
    Country     string
    Province    string
    City        string
    DisplayName string
    Source      string
    Custom      bool
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:

```bash
go test ./internal/ipmeta ./... -v
```

Expected: custom CIDR wins over builtin, and builtin fallback still resolves a country.

- [ ] **Step 5: Commit**

```bash
git add internal/ipmeta scripts/update-ipmeta.sh
git commit -m "feat: add ip geo resolution"
```

---

### Task 6: Build the Web UI, authentication, and API handlers

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/session.go`
- Create: `internal/http/router.go`
- Create: `internal/http/handlers.go`
- Create: `internal/http/handlers_test.go`
- Create: `web/templates/base.html`
- Create: `web/templates/login.html`
- Create: `web/templates/dashboard.html`
- Create: `web/templates/tasks.html`
- Create: `web/templates/ip_ranges.html`
- Create: `web/templates/settings.html`
- Create: `web/static/app.css`
- Create: `web/static/app.js`

**Interfaces:**
- Consumes:
  - `store.Query`.
  - `importer.ImportDirectory`.
  - `jobs.List`.
  - `auth.SessionManager`.
- Produces:
  - `func NewRouter(deps Deps) http.Handler`.
  - `func (h *Handlers) Login(w http.ResponseWriter, r *http.Request)`.
  - `func (h *Handlers) Query(w http.ResponseWriter, r *http.Request)`.
  - `func (h *Handlers) Tasks(w http.ResponseWriter, r *http.Request)`.
  - `func (h *Handlers) Import(w http.ResponseWriter, r *http.Request)`.

- [ ] **Step 1: Write the failing test**

```go
func TestLoginSetsSessionCookieAndProtectsQuery(t *testing.T) {
    srv := testServer(t)

    resp := postForm(t, srv.URL+"/login", url.Values{
        "username": {"admin"},
        "password": {"change-me"},
    })
    if resp.StatusCode != http.StatusSeeOther {
        t.Fatalf("status = %d", resp.StatusCode)
    }
    if len(resp.Cookies()) == 0 {
        t.Fatal("missing session cookie")
    }

    resp = getWithCookie(t, srv.URL+"/", resp.Cookies()[0])
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("status = %d", resp.StatusCode)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/http -run TestLoginSetsSessionCookieAndProtectsQuery -v
```

Expected: fail because router, auth, and templates are not implemented.

- [ ] **Step 3: Write the minimal implementation**

```go
type SessionManager struct {
    Secret []byte
}

func (m SessionManager) Sign(username string) (string, error)
func (m SessionManager) Verify(token string) (string, error)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:

```bash
go test ./internal/http ./internal/auth ./... -v
```

Expected: login works, protected routes reject unauthenticated requests, dashboard renders a usable query workspace.

- [ ] **Step 5: Commit**

```bash
git add internal/auth internal/http web
git commit -m "feat: add web ui and auth"
```

---

### Task 7: Add async export jobs and ClickHouse-backed job state

**Files:**
- Create: `internal/jobs/jobs.go`
- Create: `internal/jobs/jobs_test.go`
- Create: `internal/exporter/csv.go`
- Create: `internal/exporter/log.go`
- Create: `internal/exporter/worker.go`
- Create: `internal/exporter/exporter_test.go`

**Interfaces:**
- Consumes:
  - `store.Query`.
  - `jobs.JobStore`.
  - `query.LogFilter`.
- Produces:
  - `type Job struct`.
  - `type JobStore interface`.
  - `type Exporter struct`.
  - `func (e *Exporter) EnqueueCSV(ctx context.Context, f query.LogFilter) (string, error)`.
  - `func (e *Exporter) RunWorker(ctx context.Context) error`.

- [ ] **Step 1: Write the failing test**

```go
func TestCSVExportWritesHeaderAndRows(t *testing.T) {
    got := runCSVExport(t, []model.LogRow{
        sampleRow("140.205.70.178", "58.216.48.6"),
    })
    if !strings.Contains(got, "目的IP") || !strings.Contains(got, "58.216.48.6") {
        t.Fatalf("csv = %s", got)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/exporter -run TestCSVExportWritesHeaderAndRows -v
```

Expected: fail because exporter code is not implemented.

- [ ] **Step 3: Write the minimal implementation**

```go
type Job struct {
    ID        string
    Type      string
    Status    string
    Source    string
    Progress  int64
    Total     int64
    Error     string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:

```bash
go test ./internal/jobs ./internal/exporter ./... -v
```

Expected: jobs can be queued, marked done/failed, CSV and `.log` exports are generated from ClickHouse-backed query results.

- [ ] **Step 5: Commit**

```bash
git add internal/jobs internal/exporter
git commit -m "feat: add async exports"
```

---

### Task 8: Wire the Docker smoke test and end-to-end verification

**Files:**
- Create: `scripts/seed-sample.sh`
- Create: `scripts/smoke.sh`
- Create: `scripts/smoke-clickhouse.sh`
- Modify: `docker-compose.yml`
- Modify: `README.md`

**Interfaces:**
- Consumes:
  - the built Docker images.
  - the sample NAT log fixture.
- Produces:
  - a repeatable smoke script that verifies login, import, query, geo label, and export.

- [ ] **Step 1: Write the failing smoke script assertions**

```bash
#!/usr/bin/env bash
set -euo pipefail

base_url="${BASE_URL:-http://127.0.0.1:8080}"

curl -fsS "$base_url/health" >/dev/null
login_cookie=$(mktemp)
curl -fsS -c "$login_cookie" -d 'username=admin&password=change-me' "$base_url/login" >/dev/null
curl -fsS -b "$login_cookie" "$base_url/api/logs?start=2026-04-28&end=2026-04-28&ip=140.205.70.178&field=dst" | jq -e '.rows | length > 0'
curl -fsS -b "$login_cookie" "$base_url/api/exports" >/dev/null
```

- [ ] **Step 2: Run the smoke script to verify it fails**

Run:

```bash
bash scripts/smoke.sh
```

Expected: fail until the container, handlers, and import path are fully wired.

- [ ] **Step 3: Write the minimal implementation**

```yaml
services:
  clickhouse:
    image: clickhouse/clickhouse-server:24
    ports:
      - "8123:8123"
    volumes:
      - clickhouse_data:/var/lib/clickhouse

  zlog:
    build: .
    ports:
      - "8080:8080"
    environment:
      - CLICKHOUSE_URL=http://clickhouse:8123
    depends_on:
      - clickhouse

volumes:
  clickhouse_data:
```

- [ ] **Step 4: Run the tests to verify they pass**

Run:

```bash
docker compose up --build -d
go test ./...
bash scripts/smoke.sh
```

Expected: end-to-end import/query/export succeeds and the smoke script exits 0.

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml scripts README.md
git commit -m "chore: add end-to-end smoke coverage"
```

---

## Coverage Check

- Data model and ClickHouse schema: Tasks 1, 3, 4, 7.
- Filename parsing and log parsing: Task 2.
- Import pipeline and dedupe: Tasks 2, 4, 8.
- Destination IP geo labeling: Task 5.
- Web UI and authentication: Task 6.
- Async export and job state: Task 7.
- Docker deployment and smoke verification: Tasks 1, 8.
- Capacity and performance constraints: Tasks 1, 3, 8.


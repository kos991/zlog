# zlog — 轻量 NAT 日志 SIEM

基于 ClickHouse 的深信服防火墙 NAT 日志查询、留存和导出系统。

## 特性

- 结构化解析深信服 NAT 日志，只存结构化字段，不存原始行
- ClickHouse 列式存储，按时间 + 目的 IP 快速查询
- 支持 源 IP / 目的 IP / 转换后 IP / 全部 IP 查询
- 目的 IP 地理归属标注，支持自定义 CIDR 覆盖
- Web 界面登录后查询，支持分页和异步导出
- Docker Compose 一键部署

## 快速开始

```bash
# 准备示例数据
bash scripts/seed-sample.sh data

# 启动服务
docker compose up --build -d

# 运行冒烟测试
bash scripts/smoke.sh
```

服务启动后访问 `http://localhost:8080`。

默认账号：`admin` / `change-me`

## 配置

配置文件路径默认 `/etc/zlog/config.yaml`，可通过 `-config` 参数指定。

关键配置项见 `config.example.yaml`。

## 架构

```
浏览器 → zlog Web/API → ClickHouse
                ↑
   深信服 NAT 日志（gzip 归档）
```

- **parser**：解析文件名和 NAT 日志行
- **store**：封装 ClickHouse 读写
- **importer**：扫描、去重、批量导入
- **query**：SQL 查询构建器
- **exporter**：异步 CSV/Log 导出
- **ipmeta**：IP 地理归属解析
- **auth**：bcrypt 密码 + HMAC 会话

## 数据模型

主表 `nat_logs` 只存结构化字段，不存 `raw_line`：

- 分区：`PARTITION BY toYYYYMM(log_date)`
- 排序键：`ORDER BY (log_date, dst_ip, ts)`
- 压缩：`Delta` + `ZSTD(3)`

## 开发

```bash
# 运行单元测试
go test ./internal/... -v

# 构建二进制
go build -o zlog ./cmd/zlog

# 运行冒烟测试
docker compose up --build -d
bash scripts/smoke.sh
```

FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/zlog ./cmd/zlog

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/zlog /usr/local/bin/zlog
COPY config.example.yaml /etc/zlog/config.yaml
COPY clickhouse/init.sql /opt/zlog/init.sql

RUN mkdir -p /var/lib/zlog/exports /var/log/zlog /data/sangfor_fw_log

EXPOSE 8080

ENTRYPOINT ["zlog", "-config", "/etc/zlog/config.yaml"]

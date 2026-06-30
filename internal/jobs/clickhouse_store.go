package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouseJobStore struct {
	conn driver.Conn
}

func NewClickHouseJobStore(conn driver.Conn) *ClickHouseJobStore {
	return &ClickHouseJobStore{conn: conn}
}

func (s *ClickHouseJobStore) Create(ctx context.Context, job Job) error {
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	return s.conn.Exec(ctx,
		"INSERT INTO zlog.jobs (id, type, status, source, progress, total, error, result_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		job.ID, job.Type, job.Status, job.Source, job.Progress, job.Total, job.Error, job.ResultPath, job.CreatedAt, job.UpdatedAt,
	)
}

func (s *ClickHouseJobStore) Update(ctx context.Context, job Job) error {
	job.UpdatedAt = time.Now()
	return s.conn.Exec(ctx,
		"INSERT INTO zlog.jobs (id, type, status, source, progress, total, error, result_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		job.ID, job.Type, job.Status, job.Source, job.Progress, job.Total, job.Error, job.ResultPath, job.CreatedAt, job.UpdatedAt,
	)
}

func (s *ClickHouseJobStore) Get(ctx context.Context, id string) (Job, error) {
	var j Job
	err := s.conn.QueryRow(ctx,
		"SELECT id, type, status, source, progress, total, error, result_path, created_at, updated_at FROM zlog.jobs WHERE id = ? ORDER BY updated_at DESC LIMIT 1",
		id,
	).Scan(&j.ID, &j.Type, &j.Status, &j.Source, &j.Progress, &j.Total, &j.Error, &j.ResultPath, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

func (s *ClickHouseJobStore) List(ctx context.Context, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.conn.Query(ctx,
		fmt.Sprintf("SELECT id, type, status, source, progress, total, error, result_path, created_at, updated_at FROM zlog.jobs FINAL ORDER BY created_at DESC LIMIT %d", limit),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Type, &j.Status, &j.Source, &j.Progress, &j.Total, &j.Error, &j.ResultPath, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

package jobs

import (
	"context"
	"time"
)

type Job struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Source     string    `json:"source"`
	Progress   uint64    `json:"progress"`
	Total      uint64    `json:"total"`
	Error      string    `json:"error"`
	ResultPath string    `json:"result_path"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type JobStore interface {
	Create(ctx context.Context, job Job) error
	Update(ctx context.Context, job Job) error
	Get(ctx context.Context, id string) (Job, error)
	List(ctx context.Context, limit int) ([]Job, error)
}

func NewID() string {
	return time.Now().UTC().Format("20060102150405") + "-" + randomHex(4)
}

func randomHex(n int) string {
	const chars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
	}
	return string(b)
}

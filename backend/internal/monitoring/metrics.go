package monitoring

import (
	"context"
	"time"
)

type HostMetrics struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUPercent  float64   `json:"cpu_percent"`
	RAMUsed     int64     `json:"ram_used_bytes"`
	RAMTotal    int64     `json:"ram_total_bytes"`
	DiskUsed    int64     `json:"disk_used_bytes"`
	DiskTotal   int64     `json:"disk_total_bytes"`
	LoadAverage [3]float64 `json:"load_average"`
}

type MetricsCollector interface {
	CollectHostMetrics(ctx context.Context) (*HostMetrics, error)
}

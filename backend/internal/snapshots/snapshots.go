package snapshots

import "time"

// Snapshot represents a point-in-time state capture of a workload/instance.
type Snapshot struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	WorkloadID   string    `json:"workload_id"`
	WorkloadName string    `json:"workload_name"`
	Stateful     bool      `json:"stateful"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	Status       string    `json:"status"`
	Description  string    `json:"description,omitempty"`
}

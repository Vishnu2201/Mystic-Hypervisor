package snapshots

import "time"

type Snapshot struct {
	Name       string    `json:"name"`
	InstanceID string    `json:"instance_id"`
	CreatedAt  time.Time `json:"created_at"`
}

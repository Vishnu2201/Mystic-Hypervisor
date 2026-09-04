package storage

import "context"

type StorageVolume struct {
	Name      string `json:"name"`
	Pool      string `json:"pool"`
	Type      string `json:"type"` // container, custom, image, virtual-machine
	SizeBytes int64  `json:"size_bytes"`
}

type StorageService interface {
	ListVolumes(ctx context.Context, pool string) ([]StorageVolume, error)
}

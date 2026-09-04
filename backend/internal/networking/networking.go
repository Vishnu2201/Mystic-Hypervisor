package networking

import "context"

type Interface struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	IPAddresses []string `json:"ip_addresses"`
	MAC       string   `json:"mac"`
	IsUp      bool     `json:"is_up"`
}

type NetworkService interface {
	ListInterfaces(ctx context.Context) ([]Interface, error)
}

package tunnel

type TunnelConfig struct {
	Enabled        bool   `json:"enabled"`
	GatewayURL     string `json:"gateway_url"`
	HostID         string `json:"host_id"`
	SharedAuthKey  string `json:"-"` // Secrets must not be serialized to JSON
}

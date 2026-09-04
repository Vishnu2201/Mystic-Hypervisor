package interfaces

// Capability represents specific virtualization features supported by a provider.
type Capability string

const (
	CapVM            Capability = "vm"
	CapContainer     Capability = "container"
	CapLiveMigration Capability = "live_migration"
	CapStoragePools  Capability = "storage_pools"
	CapSnapshots     Capability = "snapshots"
	CapConsoleStream Capability = "console_stream"
	CapExec          Capability = "exec"
	CapResize        Capability = "resize"
	CapCloudInit     Capability = "cloud_init"
)

// CapabilitySet manages provider feature capabilities cleanly.
type CapabilitySet map[Capability]bool

// NewCapabilitySet creates a new set with given capabilities.
func NewCapabilitySet(caps ...Capability) CapabilitySet {
	cs := make(CapabilitySet)
	for _, c := range caps {
		cs[c] = true
	}
	return cs
}

// Has checks whether a specific capability is present.
func (cs CapabilitySet) Has(cap Capability) bool {
	if cs == nil {
		return false
	}
	return cs[cap]
}

// List returns a slice of all supported capabilities.
func (cs CapabilitySet) List() []Capability {
	list := make([]Capability, 0, len(cs))
	for cap, supported := range cs {
		if supported {
			list = append(list, cap)
		}
	}
	return list
}

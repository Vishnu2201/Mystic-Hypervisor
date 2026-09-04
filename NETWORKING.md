# Mystic Hypervisor — Network Exposure Configuration & Port Allocation Model

**Status:** Milestone 4 — Workload Networking & Port Allocation Engine  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Architectural Principles & Network Model

Mystic Hypervisor enforces a strict architectural separation between three network domains:

1. **Detected Network Facts (Read-Only Telemetry):** Empirical facts observed on the host system (e.g. interface IPs, routes, upstream NAT detection).
2. **Administrator Configuration (User Intent):** Explicit exposure choices made by administrators (e.g. exposure mode, upstream gateway settings, port forwarding rules, port allocation requests).
3. **Actual / Applied Network State (System Operations):** Applied host interface bindings, local firewall rules, and active proxy routes.

```text
┌──────────────────────────────────────┐     ┌──────────────────────────────────────┐
│       DETECTED NETWORK FACTS         │     │     ADMINISTRATOR CONFIGURATION      │
│  - Interface IPv4 (10.0.0.25)        │     │  - ExposureMode (NAT_FORWARDED)      │
│  - Host Public IP (NOT_ASSIGNED)     │     │  - Target Scope (HOST / WORKLOAD)    │
│  - Upstream Public IP (51.162.178.x) │ ──► │  - Gateway ID (gw-router-01)         │
│  - NAT Status (LIKELY)               │     │  - Gateway Public IP (51.162.178.x)  │
│  - Bridges (docker0, incusbr0)       │     │  - ForwardingRules ([]Rule)          │
└──────────────────────────────────────┘     └──────────────────────────────────────┘
                                                                │
                                                                ▼
                                             ┌──────────────────────────────────────┐
                                             │      ACTUAL / APPLIED STATE          │
                                             │  - Active Listener Bindings          │
                                             │  - Validated Interface Endpoints     │
                                             └──────────────────────────────────────┘
```

## 2. First-Class Exposure Modes

Mystic Hypervisor supports four explicit exposure modes plus a default unconfigured state:

| Exposure Mode | Enum Value | Description | Typical Use Case |
| :--- | :--- | :--- | :--- |
| **Default / Unconfigured** | `UNCONFIGURED` | No explicit exposure topology selected; system relies strictly on read-only facts. | Fresh installation / dry-run. |
| **Private Only** | `PRIVATE_ONLY` | Host & workloads reachable exclusively via internal network or VPN tunnel. | Internal clusters, WireGuard / Tailscale VPNs. |
| **NAT Forwarded** | `NAT_FORWARDED` | Private host behind an upstream NAT router/firewall forwarding specific ports. | Cloud VPS (Hetzner/OVH/AWS), home lab routers. |
| **Direct Public** | `DIRECT_PUBLIC` | Globally routable public IPv4 address assigned directly to host interface. | Bare-metal servers with public IPv4 block. |
| **External Gateway** | `EXTERNAL_GATEWAY` | Traffic routed through a dedicated external proxy or gateway server. | Cloudflare Tunnels, edge HAProxy, secondary VPS gateway. |

## 3. Upstream Gateway & Forwarding Rule Models

### A. Gateway Types (`GatewayType`) & Management Capability
- **Types**: `UPSTREAM_PROVIDER`, `EXTERNAL_ROUTER`, `EXTERNAL_VPS`, `MYSTIC_HOST`, `DEDICATED_GATEWAY`, `CLOUD_GATEWAY`, `UNKNOWN`.
- **Management Capabilities**:
  - `MANAGED_BY_MYSTIC`: Mystic has credentials/agent capability to apply rules directly to the gateway.
  - `EXTERNALLY_MANAGED`: Upstream router/proxy is managed externally by the user or cloud provider. Mystic records intent but does NOT claim the gateway has been altered.
  - `UNKNOWN`: Unclassified management state.

### B. Forwarding Rule Struct (`ForwardingRule`)
```go
type ForwardingRule struct {
	ID                string        `json:"id"`
	GatewayID         string        `json:"gateway_id,omitempty"`
	PublicIP          string        `json:"public_ip"`
	PublicPort        int           `json:"public_port"`
	Protocol          Protocol      `json:"protocol"` // TCP, UDP, TCP_UDP
	DestinationHostID string        `json:"destination_host_id"`
	DestinationIP     string        `json:"destination_ip"`
	DestinationPort   int           `json:"destination_port"`
	WorkloadID        string        `json:"workload_id,omitempty"`
	State             ExposureState `json:"state"` // UNCONFIGURED, CONFIGURED, REQUESTED, APPLIED, VERIFIED, FAILED
	Owner             ResourceOwner `json:"owner"`
	Description       string        `json:"description,omitempty"`
}
```

## 4. Port Allocation Modes & Validation Engine (Milestone 4)

Mystic Hypervisor provides an Allocation Engine (`backend/internal/networking/allocator.go`) supporting three administrator-selected allocation modes:

1. **`SINGLE` (Single Port Mapping)**: Maps a single external port (e.g. `20022`) to a single internal port (e.g. `22`).
2. **`RANGE` (Consecutive Port Range)**: Maps a range of external ports (e.g. `20022–20100`) 1:1 to an equal-sized range of internal ports (e.g. `20022–20100`). Also supports automatic consecutive range allocation from a configured `AllocationPool` (returns `ALLOCATION_POOL_UNCONFIGURED` if unconfigured).
3. **`EXPLICIT` (Explicit Mappings List)**: Maps custom individual external ports/IPs to specific workload internal ports.

### Conflict Classification Hierarchy
- `AVAILABLE`: Port is free for allocation.
- `ALREADY_ALLOCATED_BY_MYSTIC`: Claimed by another Mystic forwarding rule.
- `LISTENING_ON_HOST`: Currently bound by a process on the host (`ss -tulpn`).
- `RESERVED_MANAGEMENT`: Reserved for SSH management (port 22) or control plane API (8443).
- `OWNED_BY_EXTERNAL_SUBSYSTEM`: Owned by Docker, Pterodactyl, Incus, etc.
- `ALLOCATION_POOL_UNCONFIGURED`: Automatic pool allocation requested but no pool is configured.

## 5. Forwarding Rule Lifecycle

```text
UNCONFIGURED ──► CONFIGURED ──► REQUESTED ──► APPLIED ──► VERIFIED
                                  │             │
                                  ▼             ▼
                                FAILED        FAILED
```
- **`CONFIGURED`**: Initial state when administrator creates rule in Mystic.
- **`REQUESTED`**: When a gateway integration agent receives rule configuration request.
- **`APPLIED`**: Confirmed by gateway.
- **`VERIFIED`**: Confirmed by connectivity check.
- **`FAILED`**: Gateway rejected or connectivity verification failed.

## 6. Network Safety Engine & Management Protection

The Network Safety Engine (`installer/modules/netsafety.sh`) guarantees:
- Primary management interface (`ens18`) is protected from deletion or binding lockouts.
- Default route gateway (`10.0.0.1`) remains untouched.
- Active SSH service path (Port 22) is preserved.
- Pre-existing bridges (`docker0`, `incusbr0`, `pterodactyl0`) are preserved.
- **Zero** host network, route, or firewall mutations during dry-run or inspection.



# Mystic Hypervisor — Networking, IP Ownership & NAT Topology Model

**Status:** Milestone 3D — Public IP Ownership, NAT Awareness & Port Forwarding Model  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Overview & IP Ownership Semantics

Mystic Hypervisor strictly distinguishes between IPv4 addresses assigned to local server interfaces and addresses observed externally through Internet-facing lookups. An external lookup result (e.g. `curl https://api.ipify.org`) is evidence of an **Upstream Gateway Public IP**, but does NOT prove local interface IP ownership.

| Concept | Field | Example | Semantics |
| :--- | :--- | :--- | :--- |
| **Private IP** | `DETECTED_PRIVATE_IP` | `10.0.0.250` | RFC1918 IPv4 address assigned to local interface (`ens18`) |
| **Host Public IP** | `DETECTED_HOST_PUBLIC_IP` | `NOT_ASSIGNED` | Globally routable IPv4 address *actually assigned to local interface* |
| **Upstream Public IP** | `DETECTED_UPSTREAM_PUBLIC_IP` | `51.162.178.199` | Public IPv4 observed externally via Internet-facing lookup |
| **IP Assignment Status** | `PUBLIC_IP_ASSIGNMENT_STATUS` | `NOT_ASSIGNED` | `DIRECT` (assigned to local IF) \| `NOT_ASSIGNED` \| `UNKNOWN` |
| **NAT Status** | `NAT_STATUS` | `LIKELY` | `NOT_DETECTED` (when direct public) \| `LIKELY` (behind NAT) \| `UNKNOWN` |

## 2. Network Exposure Topologies

### A. NAT-Forwarded Topology (Private Host Behind Upstream Gateway)
```text
Internet
   │
   ▼
Upstream Public Gateway
51.162.178.199
   │
   │ NAT / Port Forwarding
   │
10.0.0.250 (ens18)
Mystic Host
   ├── Incus
   ├── Docker
   └── Mystic Workloads
```

### B. Direct-Public Topology (Public IP Assigned Directly to Host)
```text
Internet
   │
   ▼
Public IP Assigned Directly (e.g. 51.162.178.199 on ens18)
   │
Mystic Host
```

### C. Private-Only Topology
```text
Private Subnet (10.0.0.0/8 or VPN)
   │
   ▼
Private Mystic Host (10.0.0.250) (No public IP / no inbound NAT)
```

## 3. Why External IP Lookups Do Not Prove Local IP Ownership

- An external HTTP lookup reflects the WAN IP of the edge router or firewall performing Source NAT for outbound traffic.
- Storing an externally observed WAN IP as `DETECTED_HOST_PUBLIC_IP` causes severe network misconfigurations:
  1. Local services attempting to bind to the public IP will fail with `EADDRNOTAVAIL` (`cannot assign requested address`).
  2. Mystic cannot configure firewall rules on an interface for an IP it does not own.
  3. Mystic cannot claim an upstream public port is available without controlling the upstream router/gateway.

## 4. Port Forwarding Conceptual Data Model

Mystic Hypervisor prepares the data model for future port forwarding assistance without automatically altering upstream routers or local iptables rules:

```text
Forwarding Rule Concept:
  Upstream Gateway IP:  51.162.178.199
  External Port:        22022
        │
        ▼ (NAT Forwarding)
  Internal Host IP:     10.0.0.250
  Internal Port:        22
  Protocol:             TCP
```

- **Non-Identical Ports**: External ports (e.g., `22022`) do not need to match internal service ports (`22`).
- **External Gateway Boundary**: Upstream NAT configuration requires manual gateway port forwarding or explicit provider gateway integration. Mystic does NOT manipulate upstream routers or assume port availability.

## 5. Management Network Safety & Lockout Prevention

The Network Safety Engine (`installer/modules/netsafety.sh`) preserves:
- Primary management interface (`ens18`).
- Default route gateway (`10.0.0.1`).
- Active SSH service connection path (Port 22).
- Existing external bridges (`docker0`, `pterodactyl0`, `incusbr0`).

The NAT/Public IP detection model performs **zero** route additions, interface tear-downs, port bindings, or firewall mutations.

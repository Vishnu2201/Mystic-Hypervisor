# Mystic Hypervisor — Networking Architecture

**Status:** Milestone 2 — Safe Installer Foundation  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Overview

Mystic Hypervisor manages instance networking across several deployment topologies:

1. **Linux Bridge Mode**: Direct bridge attachment for external network connectivity.
2. **NAT Mode**: Isolated internal bridge with outbound NAT for private container/VM networks.
3. **Public IPv4/IPv6 Direct**: Public IP assignment when host interface routing permits.
4. **Private IP & Public Gateway Topology**: First-class support for hosts that do NOT possess a public IP.
5. **Outbound Encrypted Tunnel (Optional)**: TLS tunnel from private host to public gateway.

## 2. Private-IP VPS Architecture Preparation

Hosts situated on private subnets (e.g., 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, CGNAT) remain fully operational without requiring direct Internet-facing public IPs:

```text
Private Mystic Host (192.168.x.x)
        │
   outbound TLS
        │
        ▼
Public Mystic Gateway (203.x.x.x)
        │
     Internet
```

## 3. Network Safety & SSH Lockout Prevention

- **Preserve SSH Connection**: The installer Network Safety Engine (`installer/modules/netsafety.sh`) inspects the active default route, default management interface, SSH listening port, and active administrator session before performing network operations.
- **Safety Checks**: Operations that threaten active management routes generate explicit critical warnings and block automated application unless explicitly confirmed.

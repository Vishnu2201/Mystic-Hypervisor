# Mystic Hypervisor — Networking Architecture

**Status:** Milestone 1 — Engineering Foundation  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## Overview

Mystic Hypervisor manages instance networking across several deployment modes:

1. **Linux Bridge Mode**: Direct bridge attachment for external network connectivity.
2. **NAT Mode**: Isolated internal bridge with outbound NAT for private container/VM networks.
3. **Public IPv4/IPv6 Direct**: Public IP assignment when host interface routing permits.
4. **Private IP & Public Gateway**: Port-forwarding or reverse proxy setup for NATed/private VPS installations.
5. **Outbound Encrypted Tunnel (Optional)**: TLS tunnel from private host to public gateway.

## Safety Directives

- **Preserve SSH Access**: The installer and network configuration subsystem must never tear down or reconfigure the active management interface while connected over SSH.
- **Dry-Run Inspection**: Interface detection checks public vs private IP status without altering routing tables or iptables rules.

# Mystic Hypervisor — Safe Installer Architecture & Deep Inventory Reference

**Status:** Milestone 4 — Workload Networking & Port Allocation Engine  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Overview

The Mystic Hypervisor installer is a production-grade POSIX shell pipeline engineered around **safety, deep infrastructure inventory, resource ownership attribution (`HOST_RESOURCE_OWNER`), coexistence with pre-existing host workloads, idempotency, preflight validation, explicit user consent, transaction tracking, rollback, and failure recovery**.

It is designed to perform **100% read-only** deep system inspection across Linux target servers (Ubuntu, Debian, RHEL, Rocky Linux, AlmaLinux).

## 2. Resource Ownership Attribution (`HOST_RESOURCE_OWNER`)

To ensure Mystic Hypervisor never assumes sole ownership of pre-existing server infrastructure, every detected bridge, storage pool, and hypervisor engine is tagged with an explicit ownership model:

| Owner Enum | Description | Example Target | Mystic Action |
| :--- | :--- | :--- | :--- |
| **`SYSTEM`** | Host operating system & management route | `ens18`, `/proc`, `/sys` | Protected & preserved |
| **`INCUS`** | Incus hypervisor, networks, storage, projects | `incusbr0`, storage pools | Preserved & integrated |
| **`DOCKER`** | Docker daemon, networks, containers | `docker0`, `/var/lib/docker` | Preserved & untouched |
| **`PTERODACTYL`** | Pterodactyl Wings / Panel game servers | `pterodactyl0`, `/srv/daemon-data` | Preserved & untouched |
| **`LIBVIRT`** | libvirt QEMU/KVM instances | `virbr0` | Preserved & untouched |
| **`MYSTIC`** | Mystic Hypervisor control plane & bridges | `mysticd`, `mysticbr0` | Managed by Mystic |
| **`UNKNOWN`** | Unclassified pre-existing resources | External custom bridges | Preserved as external |

## 3. Network Exposure & JSON Plan Schema (Milestone 3E)

When executed via `./install.sh --plan --json`, the installer generates a structured JSON plan separating `"detected"` network facts from `"configuration"` settings:

```json
{
  "installer_version": "0.5.0-milestone3e",
  "networking": {
    "detected": {
      "management_interface": "ens18",
      "private_ip": "10.0.0.25",
      "host_public_ip": "NOT_ASSIGNED",
      "upstream_public_ip": "51.162.178.199",
      "public_ip_assignment_status": "NOT_ASSIGNED",
      "nat_status": "LIKELY",
      "topology": "NAT_LIKELY"
    },
    "configuration": {
      "exposure_mode": "UNCONFIGURED",
      "gateway_id": "",
      "gateway_public_ip": "",
      "forwarding_rules": []
    }
  }
}
```

This ensures zero ambiguity between empirical host state and administrator exposure configuration.


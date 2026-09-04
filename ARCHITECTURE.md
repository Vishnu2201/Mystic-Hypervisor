# Mystic Hypervisor — System Architecture

**Status:** Milestone 3B — Coexistence & Read-Only Inspection  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Overview

Mystic Hypervisor is structured as a compiled Go control plane daemon (`mysticd`), a CLI interface (`mysticctl`), a web interface (React + TypeScript), and a provider abstraction layer that interfaces directly with underlying Linux hypervisors.

```text
                         Mystic Web UI
                              |
                           HTTPS/WSS
                              |
                      Mystic Server (mysticd)
                              |
              +---------------+---------------+
              |               |               |
            Auth           Monitoring       Audit
              |               |               |
              +---------------+---------------+
                              |
                     Provider Abstraction
                              |
              +---------------+---------------+
              |               |               |
            Incus            LXC           KVM/QEMU
              |
        +-----+------+
        |            |
    Containers      VMs
```

## 2. Infrastructure Coexistence & Resource Ownership

A core architectural principle of Mystic Hypervisor is **Infrastructure Coexistence**:

- **Pre-Existing Host Infrastructure**: Mystic assumes it operates on shared or pre-configured Linux servers. Existing hypervisors (Incus, LXC, Docker, Pterodactyl) and network bridges (`docker0`, `pterodactyl0`, `incusbr0`) are treated as `EXTERNAL` / `PRESERVED` resources.
- **Resource Ownership Model (`HOST_RESOURCE_OWNER`)**:
  - `UNKNOWN` (Default for unclassified host assets)
  - `SYSTEM` (Host OS & primary management interfaces e.g. `ens18`)
  - `DOCKER` (Docker daemon & `docker0`)
  - `PTERODACTYL` (Pterodactyl panel/wings & `pterodactyl0`)
  - `INCUS` (Incus daemon, `incusbr0`, Incus storage pools/projects)
  - `MYSTIC` (Mystic control plane daemon & `mysticbr0`)
  - `EXTERNAL` (Third-party bridges & storage pools)

## 3. Provider Authoritative State

- **Mystic Database**: Retains metadata (UUID, user-configured name, ownership, tags, resource limits).
- **Virtualization Provider**: Authoritative for real-time lifecycle states (`RUNNING`, `STOPPED`, `FROZEN`, `ERROR`, `CREATING`, `DELETING`).
- **State Reconciliation**: The Reconciler resolves discrepancies dynamically by trusting live hypervisor responses over database metadata.

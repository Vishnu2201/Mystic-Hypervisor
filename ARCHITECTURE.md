# Mystic Hypervisor — System Architecture

**Status:** Milestone 5 — Real Incus Workload Provisioning & Provider Execution  
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

## 3. Workload Networking & Port Allocation Architecture (Milestone 4)

Mystic Hypervisor decouples empirical telemetry observation from administrator exposure choices:

1. **Detected Network Facts**: Empirical host observations (interface IPs, default route, NAT detection).
2. **Administrator Configuration**: Explicit exposure choices (`PRIVATE_ONLY`, `NAT_FORWARDED`, `DIRECT_PUBLIC`, `EXTERNAL_GATEWAY`, `UNCONFIGURED`).
3. **Port Allocation Engine**: Evaluates port requests (`SINGLE`, `RANGE`, `EXPLICIT`), performs conflict classification (`AVAILABLE`, `ALREADY_ALLOCATED_BY_MYSTIC`, `LISTENING_ON_HOST`, `RESERVED_MANAGEMENT`, `ALLOCATION_POOL_UNCONFIGURED`), and normalizes 1:1 mappings.
4. **Gateway Management Model**: Distinguishes `MANAGED_BY_MYSTIC` from `EXTERNALLY_MANAGED` gateways.
5. **Forwarding Rule Lifecycle**: `UNCONFIGURED` -> `CONFIGURED` -> `REQUESTED` -> `APPLIED` -> `VERIFIED`.

## 4. Real Incus Workload Provisioning & Provider Execution (Milestone 5)

Mystic Hypervisor features a production-grade workload provisioning & lifecycle execution architecture:

1. **Incus Driver (`backend/internal/providers/incus/incus.go`)**:
   - Executes real safe `incus` CLI invocations without shell interpolation (`incus list --format json`, `incus image list --format json`, `incus launch`, etc.).
   - Returns explicit `interfaces.ErrProviderUnavailable` on non-Linux development machines or hosts without active Incus daemons.
2. **Provisioning Engine (`backend/internal/workloads/manager.go`)**:
   - Manages workload lifecycle states: `DRAFT`, `VALIDATED`, `PLANNED`, `APPROVED`, `PROVISIONING`, `RUNNING`, `STOPPED`, `FAILED`, `DRIFTED`.
   - Strictly enforces the **Explicit Approval Boundary**: `ProvisionWorkload` returns an error if invoked without prior explicit `ApprovePlan` approval.
3. **Provider Authoritative State & Reconciler**:
   - Retains workload specifications in database metadata while dynamically reconciling runtime state (`actual_state`, `sync_status`) against real hypervisor responses.
   - Flags state mismatches (`RUNNING` in DB vs `STOPPED` in provider) as `DRIFTED`.
4. **Zero Fake Data Baseline**:
   - Empty workloads state (`No workloads found`) is preserved until real provider instances are queried or created by an administrator.




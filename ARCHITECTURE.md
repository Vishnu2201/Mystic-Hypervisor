# Mystic Hypervisor — System Architecture

**Status:** Milestone 1 — Engineering Foundation  
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

## 2. Core Architectural Principles

### 2.1 Provider Authoritative State

A fundamental rule of Mystic Hypervisor is that **the database is NOT authoritative for runtime state**.

- **Mystic Database**: Retains metadata (UUID, user-configured name, ownership, tags, resource limits, assigned network/storage bindings).
- **Virtualization Provider**: Authoritative for real-time lifecycle states (`RUNNING`, `STOPPED`, `FROZEN`, `ERROR`, `CREATING`, `DELETING`).

#### State Reconciliation Architecture
When an instance list or single instance status is queried via API or UI:
1. `mysticd` fetches metadata from the local database.
2. `mysticd` queries the active provider driver (`IncusProvider`, `LXCProvider`, etc.) for actual runtime state.
3. The Reconciler resolves discrepancies (e.g. if DB metadata lists VM as active but provider reports `STOPPED`, real state `STOPPED` is returned to client and DB metadata record is synced).

### 2.2 Provider Abstraction & Capabilities

Virtualization engines vary in functionality. Mystic abstracts common operations through `Provider`, `InstanceProvider`, `StorageProvider`, `NetworkProvider`, `SnapshotProvider`, and `ImageProvider` interfaces.

Each provider explicitly declares its supported feature matrix via a capability flags system (`CapVM`, `CapContainer`, `CapLiveMigration`, `CapStoragePools`, `CapSnapshots`, `CapConsoleStream`, `CapExec`).

Operations requested on a provider that lacks the corresponding capability will immediately return a standard error (`ErrUnsupportedOperation`) rather than failing implicitly or returning fake success.

### 2.3 Async Provider Operations

Long-running operations (instance creation, image downloads, snapshot generation, storage allocation) are executed asynchronously.
- Mystic API returns a `TaskID` (`202 Accepted`).
- The task status can be monitored via polling `/api/v1/tasks/{id}` or listening to live WebSocket events `/api/v1/events`.

## 3. Subsystem Breakdown

### Backend (`backend/`)
- `cmd/mysticd`: Server process entrypoint handling signals (`SIGINT`, `SIGTERM`), initializing configuration, logging, database, provider registry, API router, and HTTP/WebSocket server.
- `internal/config`: Strongly typed configuration structures with environment variable and YAML file overrides.
- `internal/logging`: Structured logger using standard JSON formatting with an automatic secret redactor wrapper to protect API tokens, passwords, and private keys.
- `internal/providers/interfaces`: Common interfaces, capability flags, and standardized error types.
- `internal/providers/incus`: Incus provider implementation.
- `internal/providers/lxc`: LXC provider implementation.
- `internal/providers/kvm`: Direct KVM/QEMU provider implementation.
- `internal/instances`: Instance domain model and state reconciler.

### CLI (`cli/`)
- `mysticctl`: Lightweight CLI client communicating with `mysticd` via REST API or local socket. Subcommands cover `status`, `doctor`, `version`, `host`, `vm`, `container`, `network`, `storage`, `logs`, `update`, `rollback`, `uninstall`.

### Frontend (`frontend/`)
- React + TypeScript Single Page Application served by `mysticd` static file handler or standalone dev server.
- Communicates with `mysticd` using `/api/v1/` REST endpoints and `/api/v1/events` WebSockets.

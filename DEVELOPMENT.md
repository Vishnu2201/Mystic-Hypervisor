# Mystic Hypervisor — Developer Guide

**Status:** Milestone 5 — Real Incus Workload Provisioning & Provider Execution  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## Target Platform & Environment Notice

- **Target Hypervisor Environment**: Linux (Ubuntu, Debian, RHEL, Rocky, AlmaLinux) running Incus, LXC, or KVM.
- **Development Host (Windows / macOS / Linux)**: Used strictly for source editing, frontend compilation, Go compilation, type checking, and platform-independent unit testing.
- **Installer Execution**: Production host installation and live hypervisor inspection execute exclusively on Linux target hosts.

## Prerequisites

- **Go**: 1.22 or higher
- **Node.js**: 18.x or higher
- **Make**: Standard GNU make (optional but recommended)

## Monorepo Layout

- `backend/`: Go control plane daemon (`mysticd`).
- `cli/`: Go command-line tool (`mysticctl`).
- `frontend/`: React + TypeScript Single Page Application shell.
- `installer/`: POSIX shell scripts for system detection, compatibility, and dry-run setup.

## Building Locally

### Backend
```bash
cd backend
go build -v -o bin/mysticd ./cmd/mysticd
```

### CLI
```bash
cd cli
go build -v -o bin/mysticctl ./main.go
```

### Frontend
```bash
cd frontend
npm install
npm run build
```

## Milestone Roadmap

- **Milestone 1 — Engineering Foundation**: COMPLETED (Base repository structure, provider abstraction, reconciler, CLI shell, React UI shell, installer dry-run platform awareness).
- **Milestone 2 — Safe Installer**: COMPLETED (System package transactions, rollback engine, systemd service registration, plan mode, dry-run purity).
- **Milestone 3B — Real Read-Only VPS Inspection**: COMPLETED (Empirical VPS observation on Debian 13 trixie node).
- **Milestone 3C — Deep System & Infrastructure Inventory**: COMPLETED (Incus, Docker, Pterodactyl, KVM, cgroup, ownership model, dynamic recommendation engine).
- **Milestone 3D — Public IP Ownership & NAT Model**: COMPLETED (Host interface IP vs external gateway IP separation).
- **Milestone 3E — Network Exposure Configuration & Forwarding Model**: COMPLETED (Exposure modes, gateway data model, UI component, detected vs configuration JSON separation).
- **Milestone 4 — Workload Networking & Port Allocation Engine**: COMPLETED (Workload network config, allocation engine, 1:1 range preview, conflict classification, REST API endpoints, dynamic network topology diagram).
- **Milestone 5 — Real Incus Workload Provisioning & Provider Execution**: COMPLETED (Real Incus driver execution, workload domain model, pre-flight spec validation, provisioning plan generator, explicit administrator approval boundary, lifecycle controls, state reconciliation, React provisioning wizard UI).




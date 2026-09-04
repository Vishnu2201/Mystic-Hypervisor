# Mystic Hypervisor

> **Status:** Milestone 1 — Engineering Foundation  
> **Source of Truth:** `PROJECT_CONSTITUTION.md`

Mystic Hypervisor is a lightweight, self-hosted virtualization and infrastructure management platform designed to provide a Proxmox-like control experience without unnecessary footprint or complexity.

## Architecture & Principles

1. **Lightweight First**: Built using Go for the control plane and React + TypeScript for the web interface. No Docker, Kubernetes, Redis, Prometheus, or Grafana required.
2. **Real Infrastructure Only**: Operating strictly against real Linux host primitives (Incus, LXC, KVM). No fake or mock production data.
3. **Provider Authoritative State**: The database stores metadata only; the virtualization provider is authoritative for real-time VM and container runtime states.
4. **Safety Before Convenience**: Non-destructive installation, explicit dry-run mode, backup of system state, rollback capabilities, and safe uninstaller.

## Repository Layout

```text
mystic-hypervisor/
├── PROJECT_CONSTITUTION.md   # System constitution and source of truth
├── ARCHITECTURE.md           # Technical architecture & reconciliation design
├── SECURITY.md               # Security model, secrets, TLS & RBAC
├── DEVELOPMENT.md            # Developer setup & contribution guidelines
├── TESTING.md                # Testing policy & execution instructions
├── API.md                    # REST API & WebSocket specifications
├── NETWORKING.md             # Networking topologies & gateway/tunnel specs
├── INSTALLER.md              # Installer architecture & safe operations
├── Makefile                  # Monorepo build and test commands
├── backend/                  # Go daemon (mysticd)
├── cli/                      # Command line tool (mysticctl)
├── frontend/                 # React + TypeScript Web UI shell
├── installer/                # Shell installer script pipeline
├── scripts/                  # Development scripts
├── packaging/                # Systemd and packaging files
└── docs/                     # Documentation assets
```

## Quickstart (Development)

### Requirements
- Go 1.22+
- Node.js 18+ & npm

### Build
```bash
# Build backend
cd backend && go build -o bin/mysticd ./cmd/mysticd

# Build CLI
cd cli && go build -o bin/mysticctl ./main.go

# Build frontend
cd frontend && npm install && npm run build
```

### Run Unit Tests
```bash
cd backend && go test -v ./...
cd cli && go test -v ./...
```

### Installer Dry Run
```bash
bash installer/install.sh --dry-run
```

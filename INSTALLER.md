# Mystic Hypervisor — Safe Installer Architecture & Reference

**Status:** Milestone 2 — Safe Installer Foundation  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Overview

The Mystic Hypervisor installer is a production-grade POSIX shell pipeline engineered around **safety, idempotency, preflight validation, explicit user consent, transaction tracking, rollback, and failure recovery**.

It is designed strictly for target Linux servers (Ubuntu, Debian, RHEL, Rocky Linux, AlmaLinux) while maintaining explicit environment awareness when executed under development host environments (Windows / Git Bash / macOS).

## 2. Installer Modes & Invocation

Default invocation (`./install.sh`) displays safe usage instructions and requires an explicit mode flag to prevent accidental host modifications.

| Mode | Command | Description | Safety Behavior |
| :--- | :--- | :--- | :--- |
| **Default** | `./install.sh` | Explains installer capabilities and modes | Read-only / Explanatory |
| **Dry-Run** | `./install.sh --dry-run` | Performs system detection & renders execution plan | Read-only (Exit code 0) |
| **Plan** | `./install.sh --plan [--json]` | Generates human- or machine-readable plan | Read-only |
| **Apply** | `./install.sh --apply [--yes]` | Applies installation transaction to host server | Requires explicit user confirmation; blocked on non-Linux dev hosts |
| **Rollback** | `./install.sh --rollback` | Undoes recorded transaction changes | Safe restoration of Mystic-altered state |
| **Doctor** | `./install.sh --doctor` | Standalone system health diagnostic | Read-only diagnostic |
| **Uninstall** | `./install.sh --uninstall` | Safely removes Mystic control plane | Preserves user VMs, containers, & storage |
| **Version** | `./install.sh --version` | Outputs installer version information | Read-only |
| **Help** | `./install.sh --help` | Displays usage options | Read-only |

## 3. Transaction Model & State Management

Every installer operation is tracked via a deterministic Transaction ID (`TX_<timestamp>_<rand>`) recorded under `/var/lib/mystic/transactions/`.

### Transaction Life Cycle States
- `NOT_STARTED`
- `IN_PROGRESS`
- `COMPLETED`
- `FAILED`
- `PARTIALLY_COMPLETED`
- `ROLLED_BACK`

Before any configuration file modification, pre-change state manifests (`system-info.json`, `network-before.json`, `firewall-before.json`, `services-before.json`, `sshd-before.json`) are stored. Secrets (passwords, tokens, keys) are automatically redacted from transaction logs.

## 4. Resource Tiering & Classification

Based on Constitution Section 8, the installer classifies host resources into deterministic tiers:

- **Tiny Profile (< 2 GB RAM, 1 CPU)**: Recommends LXC container-only workloads. Warns against KVM VM creation.
- **Small Profile (2 - 4 GB RAM)**: Recommends Incus hypervisor.
- **Standard Profile (4 - 8 GB RAM)**: Recommends Incus + KVM.
- **Large Profile (8+ GB RAM)**: Recommends Incus + KVM.
- **Development Host (Windows / Git Bash)**: Rated `UNSUPPORTED_DEV_HOST`.

## 5. Network Safety Engine

The installer includes a Network Safety Engine (`installer/modules/netsafety.sh`) to prevent SSH lockout, default route loss, or management interface destruction:
- Identifies active management network interface, IP address, and default gateway.
- Detects running SSH service status and custom SSH listening ports.
- Guarantees that management interfaces are preserved and never silently replaced by unconfigured network bridges.

## 6. Package Manager Abstraction

Supports `apt` (Debian/Ubuntu) and `dnf`/`yum` (RHEL/Rocky/AlmaLinux) through a unified abstraction interface (`installer/modules/pkgmanager.sh`). Package installation plans calculate missing vs installed packages without executing `apt` or `dnf` during dry-run or plan generation.

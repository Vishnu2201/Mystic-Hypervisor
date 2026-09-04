# Mystic Hypervisor — Safe Installer Architecture & Deep Inventory Reference

**Status:** Milestone 3C — Deep Linux Host & Existing Infrastructure Inventory  
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

## 3. Milestone 3C Deep Read-Only Inspection Matrix

When executed via `./install.sh --dry-run` or `./install.sh --plan --json`, the detection engine inspects:

1. **Host Deep Telemetry**: Hostname, non-secret Machine ID status hash, OS release, Kernel, Architecture, Boot mode (UEFI/BIOS), Uptime, Systemd status, cgroup version (v1/v2), Virtualization environment (`systemd-detect-virt`), CPU topology (sockets/cores), CPU model, RAM, Swap, Load average, Root filesystem, Inode usage, and Block devices (`lsblk`).
2. **KVM Capability Matrix**:
   - `/dev/kvm` status: `ACCESSIBLE`, `PRESENT_INACCESSIBLE`, `NOT_PRESENT`, `UNAVAILABLE`
   - KVM Unavailable Reason: Explains missing `/dev/kvm` device nodes (e.g. cloud VPS instances without nested virtualization enabled) or udev permission errors.
   - CPU Virtualization flags (`vmx`/`svm`) & Nested virtualization status (`AVAILABLE`/`NOT_AVAILABLE`).
3. **Incus Deep Inspection (Read-Only)**: Queries `incus version`, daemon availability, storage pools & drivers, networks & types, projects, profiles, and instance counts without creating client configs or executing mutations.
4. **Docker Deep Inspection (Read-Only)**: Queries `docker --version`, daemon availability, running/total container count, network names, and volume counts without stopping/starting containers.
5. **Pterodactyl / Existing Services**: Detects `wings` daemon, Pterodactyl storage paths, and `pterodactyl0` bridge.
6. **Port & Service Inventory**: Read-only `ss -tulpn` scan identifying active listening ports (SSH 22, Incus API, Docker daemon, Pterodactyl Wings).
7. **Coexistence-Aware Provider Recommendation Engine**: Dynamically evaluates `Resource Capacity + Hardware Virt + Existing Providers + Existing Networks + Existing Storage + Ownership + Coexistence Risk`.
   - *Example*: A server with 10 CPUs, 64 GB RAM, Incus installed, but `/dev/kvm` acceleration missing, dynamically recommends **Incus** with detailed multi-point rationale.

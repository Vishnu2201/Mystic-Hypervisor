# Mystic Hypervisor — Project Constitution & Architecture

**Status:** Baseline / source of truth  
**Version:** 1.0  
**Date:** 2026-09-04

---

## 1. Purpose

Mystic Hypervisor is a lightweight, self-hosted virtualization and infrastructure management platform.

The goal is to provide a Proxmox-like experience without unnecessarily reproducing Proxmox's full footprint or complexity.

Mystic Hypervisor will provide:

- A single-command installation experience.
- Automatic host/system inspection.
- Choice of virtualization backend.
- Real VM and container lifecycle management.
- Real host and instance monitoring.
- Storage and networking management.
- REST API and CLI.
- Authentication, RBAC, and audit logging.
- Safe installation, rollback, repair, update, and uninstall.
- Public-IP and private-IP deployment support.
- Optional public gateway / port-forwarding and outbound tunnel capabilities.
- A path toward multi-host management.

**Core principle:** Mystic manages real infrastructure. It must never rely on fake, mock, seeded, simulated, or placeholder infrastructure data in production UI/API behavior.

---

# 2. Non-Negotiable Principles

These rules govern the entire project.

## 2.1 Lightweight First

Mystic must remain suitable for small servers.

Do not introduce a dependency merely because it is convenient.

Before adding a service, ask:

1. Is it necessary?
2. Can the existing platform provide the functionality?
3. What is its RAM/CPU/disk overhead?
4. Does it increase operational complexity?
5. Is there a lighter alternative?

Avoid unnecessary stacks such as Docker + Kubernetes + Redis + Prometheus + Grafana simply to implement functionality that can be provided directly.

Target a small management footprint and benchmark actual usage rather than making unsupported guarantees.

## 2.2 Real Infrastructure Only

Production functionality must operate against:

- The real Linux host.
- The real virtualization provider.
- The real storage subsystem.
- The real networking stack.
- The real database.
- Real user/API actions.

No fake VM lists, fake metrics, fake network traffic, fake IP addresses, or fake monitoring data.

## 2.3 Safety Before Convenience

The VPS used for testing is real infrastructure.

The installer must assume it can be connected to over SSH and must protect connectivity.

No destructive action should occur silently.

## 2.4 Never Destroy User Workloads Accidentally

Removing Mystic must not automatically delete:

- Existing VMs.
- Existing containers.
- Existing storage.
- Existing user data.
- Existing network configurations.

Destructive provider removal must be a separate, explicit operation.

## 2.5 Provider Agnostic

Mystic must use a virtualization-provider abstraction.

Initial providers:

- Incus.
- LXC.
- KVM/QEMU where appropriate.

Incus should be the preferred general-purpose provider because it can manage both system containers and VMs.

The UI/API should not be tightly coupled to Incus-specific concepts where an abstraction is practical.

## 2.6 Secure by Default

Security is part of the architecture, not a later feature.

The platform should provide:

- HTTPS/TLS.
- Strong authentication.
- RBAC.
- Secure sessions.
- API authentication.
- Token management.
- Rate limiting.
- Input validation.
- Security headers.
- Audit logs.
- Host identity.
- Secure remote communication.
- MFA-ready architecture.

Never expose unrestricted arbitrary shell execution through the public API.

---

# 3. Product Identity

Product name:

**Mystic Hypervisor**

The product is an independent virtualization management platform.

Do not market or implement it as a literal Proxmox clone.

Conceptually:

> Linux + Incus/KVM/LXC + Mystic control plane

---

# 4. Target Architecture

```text
                         Mystic Web UI
                              |
                           HTTPS/WSS
                              |
                      Mystic Server/API
                              |
              +---------------+---------------+
              |               |               |
            Auth           Monitoring       Audit
              |               |               |
              +---------------+---------------+
                              |
                     Provider Abstraction
                              |
             +----------------+----------------+
             |                |                |
           Incus             LXC            KVM/QEMU
             |
       +-----+------+
       |            |
   Containers       VMs
```

Mystic should preferably run as a small compiled service, with a lightweight frontend.

A Go backend is the preferred direction for the management service because of its low runtime overhead and easy single-binary deployment.

The frontend may be React + TypeScript or another lightweight modern web stack, but the final build should be deployable without requiring a large runtime stack on the production host.

---

# 5. Installation Experience

Primary goal:

```bash
curl -fsSL https://install.mysticpanel.dev | sudo bash
```

The bootstrap should obtain a versioned installer/release rather than embedding the entire project into an unmaintainable shell script.

Preferred commands after installation:

```bash
mysticctl status
mysticctl doctor
mysticctl update
mysticctl restart
mysticctl logs
mysticctl rollback
mysticctl uninstall
```

---

# 6. Installer Pipeline

The installer must follow this order:

```text
System Detection
      ↓
Compatibility Checks
      ↓
Safety / Existing-State Inspection
      ↓
Installation Plan
      ↓
User Confirmation
      ↓
Backup / Transaction State
      ↓
Dependency Installation
      ↓
Virtualization Provider
      ↓
Storage
      ↓
Networking
      ↓
Private/Public Connectivity
      ↓
Mystic Services
      ↓
Security
      ↓
Health Checks
      ↓
Finalization
```

Do not make irreversible changes before the plan and safety checks have completed.

---

# 7. System Detection

Detect at minimum:

- OS distribution.
- OS version.
- Kernel.
- Architecture.
- CPU count.
- CPU features.
- Hardware virtualization.
- KVM availability.
- Nested virtualization where detectable.
- RAM.
- Swap.
- Disk capacity.
- Disk type where detectable.
- Filesystem.
- Network interfaces.
- IPv4.
- IPv6.
- Default route.
- DNS.
- Public IP.
- Private IP.
- NAT indicators.
- Firewall.
- Existing Incus.
- Existing LXC.
- Existing KVM/QEMU.
- Other virtualization software.
- Docker/Podman where relevant.
- Existing bridges.
- Current SSH connection and source.
- Existing Mystic installation.

System suitability should be classified:

- UNSUPPORTED
- LIMITED
- SUITABLE
- RECOMMENDED
- EXCELLENT

---

# 8. Resource Profiles

Mystic should provide recommendations based on detected hardware.

## Tiny

Typical:

- 1 CPU.
- 1–2 GB RAM.
- ~25 GB storage.

Recommendation:

- LXC or minimal Incus container workload.
- Warn before VM use.

## Small

Typical:

- 2 CPU.
- 2–4 GB RAM.

Recommendation:

- Incus.

## Standard

Typical:

- 4+ CPU.
- 8+ GB RAM.

Recommendation:

- Incus + KVM.

## Large

Typical:

- 8+ CPU.
- 16+ GB RAM.

Recommendation:

- Incus + KVM.
- Advanced storage/networking where justified.

Users must be able to override recommendations after seeing the consequences.

---

# 9. Virtualization Provider Selection

Installer options:

1. Incus — recommended general-purpose option.
2. LXC — container-focused and extremely lightweight.
3. KVM/QEMU — full VM-focused deployment.
4. Incus + KVM — recommended mixed workload option.
5. Auto-select — Mystic chooses based on hardware and requirements.

The provider layer must isolate provider-specific implementation details from the API/UI.

---

# 10. Storage

Supported/possible backends:

- Directory.
- LVM.
- LVM-thin.
- ZFS.
- Btrfs.

The installer must not select an advanced storage backend merely because it exists.

It should consider:

- RAM.
- Disk type.
- Available capacity.
- Number of disks.
- Existing data.
- Snapshot requirements.
- Compression requirements.
- Performance.
- Operational complexity.

Existing disks/data must be protected.

Never reformat a disk without an explicit destructive confirmation.

---

# 11. Networking

Supported conceptual modes:

- Linux bridge.
- NAT.
- Routed.
- VLAN.
- Existing bridge.
- Custom configuration.

The installer must detect the current network before making changes.

When connected through SSH:

- Preserve the current management interface.
- Preserve the current route.
- Preserve the current gateway.
- Preserve SSH access.
- Preserve DNS where possible.
- Back up network state.
- Verify connectivity after changes.
- Abort/rollback if connectivity is lost or validation fails.

Never assume a public-IP server can safely have its primary interface replaced by a bridge.

---

# 12. Public-IP Detection

Mystic should distinguish:

### Public IPv4

Host is directly reachable if upstream firewall/routing permits.

### Public IPv6

Host may be reachable over IPv6 if routing/firewall permits.

### Private IPv4

Examples:

- 10.0.0.0/8.
- 172.16.0.0/12.
- 192.168.0.0/16.

### NAT / CGNAT / restricted environment

Inbound connectivity may be unavailable.

The UI must clearly show the actual detected state.

---

# 13. Private-IP Connectivity

A private host must remain fully usable without public inbound connectivity.

Installer/wizard:

```text
Private network detected.

Choose external connectivity:

1. Keep private.
2. Port forwarding through another public gateway.
3. Outbound encrypted tunnel to a public gateway.
4. IPv6 if externally reachable.
5. Configure later.
```

Mystic must never claim that a private IP is directly Internet-reachable.

---

# 14. Public Gateway / Port Forwarding

Architecture:

```text
Internet
   |
Public Gateway
203.x.x.x
   |
TCP port
   |
192.168.x.x
Mystic Host
```

Mystic should provide configuration assistance and validation.

A gateway may be:

- Router.
- Firewall.
- Public VPS.
- Linux host.
- Another server.
- Another Mystic host.

Port forwarding must be explicit and limited.

Do not expose the entire private network by default.

---

# 15. Outbound Tunnel

Optional architecture:

```text
Private Mystic Host
        |
   outbound TLS
        |
        v
Public Mystic Gateway
        |
     Internet
```

Requirements:

- Strong host identity.
- Mutual authentication where appropriate.
- Encryption.
- Explicit port authorization.
- Credential rotation.
- Revocation.
- Connection expiry/renewal.
- Audit logging.
- Rate limiting.
- No unrestricted private-network access.

The private host initiates the outbound connection.

---

# 16. Instance Management

VM operations:

- Create.
- Start.
- Stop.
- Shutdown.
- Restart.
- Force stop.
- Delete.
- Clone.
- Rename.
- Resize.
- Attach/detach disk.
- Attach/detach NIC.
- CPU configuration.
- Memory configuration.
- Boot order.
- Cloud-init where supported.
- Snapshot.
- Restore.
- Export/import where supported.
- Console.
- Metrics.
- Logs/events.

Container operations:

- Create.
- Start.
- Stop.
- Restart.
- Delete.
- Clone.
- Rename.
- Resize.
- Snapshot.
- Restore.
- Console.
- Exec.
- Freeze/unfreeze.
- Metrics.

All operations must act on real provider state.

---

# 17. Images

Image management:

- List.
- Search where provider supports it.
- Download.
- Upload.
- Delete.
- Verify.
- Update.
- Create from instance.
- Custom image support.

The UI must only display images that actually exist or are genuinely available through the configured provider.

---

# 18. Monitoring

Host metrics:

- CPU utilization.
- CPU count/frequency where available.
- Load average.
- RAM.
- Swap.
- Disk capacity.
- Disk I/O.
- Network RX.
- Network TX.
- Network errors.
- Filesystem usage.
- Process count.
- Uptime.
- Temperature where available.

Instance metrics:

- CPU.
- RAM.
- Disk.
- Disk I/O.
- Network RX.
- Network TX.
- Process information where supported.
- State.
- Uptime.

Initial monitoring should prioritize current/live metrics.

Historical metrics should be optional and retention-controlled.

---

# 19. Monitoring Storage

Avoid forcing a heavyweight observability stack.

Initial model:

```text
Host / Incus
     ↓
Mystic Collector
     ↓
Mystic API/WebSocket
     ↓
Browser
```

Optional historical storage:

```text
Collector
   ↓
Metrics Store
   ↓
Retention Policy
   ↓
Graphs
```

Retention should be configurable.

Example:

- 1 hour.
- 6 hours.
- 24 hours.
- 7 days.
- 30 days.
- Custom.

---

# 20. Authentication

Core entities:

- Users.
- Roles.
- Sessions.
- API tokens.
- Permissions.

Default roles:

- Administrator.
- Operator.
- Viewer.
- Custom role.

Example permissions:

```text
host.read
host.manage

instance.read
instance.create
instance.start
instance.stop
instance.delete
instance.console

network.read
network.manage

storage.read
storage.manage

user.read
user.manage

audit.read
```

---

# 21. API

API must be versioned:

```text
/api/v1/
```

Core endpoints:

```text
GET    /api/v1/hosts
GET    /api/v1/instances
POST   /api/v1/instances
GET    /api/v1/instances/{id}
PATCH  /api/v1/instances/{id}
DELETE /api/v1/instances/{id}

POST   /api/v1/instances/{id}/start
POST   /api/v1/instances/{id}/stop
POST   /api/v1/instances/{id}/restart

GET    /api/v1/instances/{id}/metrics

GET    /api/v1/images
GET    /api/v1/storage
GET    /api/v1/networks
GET    /api/v1/snapshots

GET    /api/v1/users
GET    /api/v1/audit
```

API documentation should be available through OpenAPI.

Prefer:

```text
/api/docs
```

---

# 22. CLI

Required command family:

```bash
mysticctl status
mysticctl doctor
mysticctl version

mysticctl host list

mysticctl vm list
mysticctl vm create
mysticctl vm start NAME
mysticctl vm stop NAME
mysticctl vm delete NAME

mysticctl container list
mysticctl container create

mysticctl network list
mysticctl storage list

mysticctl logs
mysticctl update
mysticctl rollback
mysticctl uninstall
```

---

# 23. Audit Logging

Audit every privileged operation.

At minimum capture:

- Timestamp.
- User/service identity.
- Source address where available.
- Action.
- Target.
- Relevant parameters, excluding secrets.
- Result.
- Error information where applicable.

Examples:

- Login.
- Failed login.
- API token creation/revocation.
- VM creation/deletion.
- VM state changes.
- Network changes.
- Storage changes.
- User/role changes.
- Configuration changes.
- Tunnel changes.

Secrets must never be written to logs.

---

# 24. Multi-Host Architecture

Future target:

```text
                 Mystic Controller
                       |
        +--------------+--------------+
        |              |              |
      Host 01        Host 02        Host 03
       Incus          Incus          Incus
        |              |              |
      VM/CT          VM/CT          VM/CT
```

Hosts should have states such as:

- ONLINE.
- DEGRADED.
- OFFLINE.
- UNKNOWN.

Remote communication must be authenticated and encrypted.

---

# 25. Scheduling / Placement

Future capability:

When creating an instance, Mystic may select a suitable host based on:

- CPU capacity.
- RAM capacity.
- Storage capacity.
- Provider capabilities.
- Network availability.
- Labels/tags.
- Affinity/anti-affinity.
- User constraints.

Do not implement scheduling before the single-host provider abstraction is stable.

---

# 26. Backups

Snapshots and backups are different.

Snapshot:

- Fast.
- Usually local.
- Short-term recovery.

Backup:

- Independent copy.
- Long-term recovery.
- Potentially remote.

Future backup targets:

- Local.
- SSH/SFTP.
- NFS.
- S3-compatible storage.
- Remote Mystic host.

Capabilities:

- Scheduling.
- Retention.
- Encryption.
- Verification.
- Restore.

---

# 27. Installer Safety

Required installer modes:

```bash
mysticctl install
mysticctl install --dry-run
```

Dry run must make no changes.

It must show:

- Detected system.
- Existing software.
- Planned package changes.
- Planned storage changes.
- Planned network changes.
- Firewall changes.
- Services to be installed.
- Expected risk.
- Warnings.

Before modifying the system, create installation state/backup information.

Example location:

```text
/var/lib/mystic/backups/
```

Possible records:

```text
system-info.json
network-before.json
firewall-before.json
services-before.json
packages-before.json
incus-before.json
mystic-install-state.json
```

Only store data necessary for recovery, and protect sensitive files.

---

# 28. Rollback

Mystic must support:

```bash
mysticctl rollback
```

Rollback should undo Mystic's own changes where safely possible.

It must not blindly revert unrelated administrator changes made after installation.

Network rollback must prioritize restoring management connectivity.

---

# 29. Uninstall

Default uninstall:

```bash
mysticctl uninstall
```

Removes:

- Mystic UI.
- Mystic API.
- Mystic CLI.
- Mystic configuration.
- Mystic database.
- Mystic services.

It must preserve by default:

- Existing VMs.
- Existing containers.
- Existing storage.
- Existing networks.
- Existing user data.

Provider removal must require separate explicit confirmation.

Any destructive operation must display exactly what will be deleted.

---

# 30. Health / Doctor System

Command:

```bash
mysticctl doctor
```

Checks:

- OS.
- Kernel.
- CPU virtualization.
- KVM.
- Provider.
- Storage.
- Network.
- DNS.
- Firewall.
- Database.
- API.
- Web UI.
- TLS.
- Services.
- Instance state.

Example result:

```text
Overall health: GOOD
```

Failures should provide diagnostics and safe suggested actions.

Doctor must not automatically make risky changes.

---

# 31. Service Architecture

Prefer as few long-running services as practical.

Target:

```text
mysticd
incusd
```

Additional services should only be introduced when technically justified.

Avoid unnecessary mandatory dependencies.

The system should remain usable without a cloud control plane.

---

# 32. Data Architecture

Mystic needs persistent state for:

- Users.
- Roles.
- Permissions.
- Sessions.
- API tokens (securely represented).
- Hosts.
- Provider configuration.
- Mystic-managed metadata.
- Audit logs.
- Optional metrics history.
- Tunnel/gateway configuration.

Provider state must remain authoritative for actual VM/container state.

Mystic's database must not become a false source of truth.

For example:

If Mystic DB says a VM is running but Incus says it is stopped, the UI must reconcile against the real provider state.

---

# 33. Frontend

Primary navigation:

```text
Dashboard
Hosts
Virtual Machines
Containers
Images
Storage
Networks
Snapshots
Monitoring
Users
API
Audit Logs
Settings
```

Dashboard should display real-time system state.

Instance pages should provide:

- Overview.
- Console.
- Metrics.
- Configuration.
- Network.
- Storage.
- Snapshots.
- Logs/events.

UI should be responsive and usable on desktop and mobile.

---

# 34. No Fake Data Rule

This rule applies to development and production.

Do not add:

- Demo VMs.
- Fake metrics.
- Mock host data in production paths.
- Seeded fake users.
- Example IPs presented as actual state.
- Simulated resource graphs.
- Placeholder instance status.

Test fixtures may exist only inside isolated automated tests and must never leak into production behavior.

---

# 35. Development Workflow

Development occurs locally.

Codex is used to implement code.

Production/test VPS changes happen only after review.

Workflow:

```text
Local development
      ↓
Code review
      ↓
Local build
      ↓
Local tests
      ↓
Package/release
      ↓
Upload to test VPS
      ↓
Dry run
      ↓
Review installation plan
      ↓
Actual installation
      ↓
Health check
      ↓
Manual E2E testing
      ↓
Fix / iterate
```

Never assume a successful local build means the VPS installation is safe.

---

# 36. VPS Testing Rules

The test VPS is real.

Before every potentially destructive test:

1. Confirm current SSH access.
2. Record current network state.
3. Confirm recovery/rescue access is available through the VPS provider when possible.
4. Run dry-run first.
5. Review planned changes.
6. Apply one category of change at a time.
7. Verify connectivity.
8. Run `mysticctl doctor`.
9. Perform manual UI/API tests.
10. Clean up test instances afterward.

---

# 37. Testing Priorities

Test in this order:

### Phase A
Installer detection.

### Phase B
Dry run.

### Phase C
Mystic installation without aggressive network changes.

### Phase D
Incus installation/configuration.

### Phase E
Create/delete container.

### Phase F
Create/delete VM.

### Phase G
Storage operations.

### Phase H
Network operations.

### Phase I
Snapshots.

### Phase J
Monitoring.

### Phase K
API.

### Phase L
Authentication/RBAC.

### Phase M
Reboot persistence.

### Phase N
Upgrade.

### Phase O
Rollback.

### Phase P
Uninstall.

### Phase Q
Private-IP gateway/tunnel functionality.

Do not jump directly to complex networking/tunneling tests on the production-like VPS.

---

# 38. Project Repository

Proposed structure:

```text
mystic-hypervisor/
│
├── README.md
├── PROJECT_CONSTITUTION.md
├── ARCHITECTURE.md
├── SECURITY.md
├── DEVELOPMENT.md
├── TESTING.md
├── API.md
├── NETWORKING.md
├── INSTALLER.md
│
├── installer/
│   ├── install.sh
│   ├── uninstall.sh
│   ├── rollback.sh
│   ├── doctor.sh
│   └── modules/
│
├── backend/
│   ├── cmd/
│   ├── internal/
│   │   ├── api/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── rbac/
│   │   ├── hosts/
│   │   ├── instances/
│   │   ├── providers/
│   │   │   ├── incus/
│   │   │   ├── lxc/
│   │   │   └── kvm/
│   │   ├── storage/
│   │   ├── networking/
│   │   ├── monitoring/
│   │   ├── snapshots/
│   │   ├── images/
│   │   ├── audit/
│   │   └── tunnel/
│   └── migrations/
│
├── frontend/
│   └── src/
│
├── cli/
│
├── agent/
│
├── docs/
│
├── scripts/
│
└── packaging/
```

The `agent/` directory should remain optional until a real technical requirement for an agent is established.

---

# 39. Implementation Order

Do not build everything simultaneously.

## Milestone 1 — Foundation

- Repository.
- Backend.
- Frontend shell.
- Configuration.
- Logging.
- Database.
- Provider abstraction.
- CLI foundation.

## Milestone 2 — Safe Installer

- Detection.
- Compatibility.
- Dry run.
- Backup/state.
- Install.
- Rollback.
- Uninstall.
- Doctor.

## Milestone 3 — Incus

- Provider implementation.
- Host connection.
- Instances.
- Images.
- Storage.
- Networks.
- Snapshots.

## Milestone 4 — Monitoring

- Host metrics.
- Instance metrics.
- WebSocket/live updates.
- Basic graphs.

## Milestone 5 — Security

- Authentication.
- RBAC.
- API tokens.
- Sessions.
- Audit logs.

## Milestone 6 — Full UI

- Dashboard.
- VM pages.
- Container pages.
- Storage.
- Network.
- Images.
- Snapshots.
- Settings.

## Milestone 7 — Networking

- Public mode.
- Private mode.
- Port-forwarding assistance.
- Gateway configuration.
- Outbound tunnel.

## Milestone 8 — Additional Providers

- LXC provider.
- KVM/QEMU provider where useful.

## Milestone 9 — Multi-host

- Remote hosts.
- Host health.
- Host management.
- Placement/scheduling.

## Milestone 10 — Backups / Advanced Platform

- Backup system.
- Remote storage.
- Advanced monitoring.
- Automated recovery.
- Additional integrations.

---

# 40. Scope Control

This document is the source of truth.

When a new feature is proposed, classify it:

### CORE
Required for the product's primary purpose.

### IMPORTANT
Useful and planned, but not blocking the foundation.

### FUTURE
Valid idea, intentionally deferred.

### OUT OF SCOPE
Does not align with Mystic Hypervisor.

Do not add features merely because another hypervisor has them.

The question is:

> Does this improve lightweight, safe, real infrastructure management?

If not, defer it.

---

# 41. Definition of Done

A feature is not complete merely because code compiles.

A feature is complete when:

1. Code is implemented.
2. Security implications are reviewed.
3. Error handling exists.
4. Real provider integration works where applicable.
5. UI behavior works.
6. API behavior works where applicable.
7. Logs/audit behavior is appropriate.
8. Failure paths are handled.
9. Manual end-to-end testing has been performed.
10. Documentation is updated.
11. No fake production data was introduced.
12. Resource usage is reasonable.

---

# 42. Codex Rules

When using Codex:

- Work from this constitution.
- Do not silently change the architecture.
- Do not replace a planned component with a heavier dependency without justification.
- Do not introduce mock production data.
- Do not claim tests were run unless they actually were.
- Do not claim VPS changes were performed unless they actually were.
- Do not make destructive infrastructure changes without explicit instruction.
- Prefer small, reviewable implementation steps.
- Update documentation when architecture changes.
- Preserve backwards compatibility where practical.
- Explain security-sensitive design decisions.
- Never expose secrets in source code or logs.

If a proposed implementation conflicts with this document, stop and identify the conflict before proceeding.

---

# 43. Change Control

This document can evolve, but architecture changes must be intentional.

For every major architectural change, document:

```text
Current approach
Proposed approach
Reason
Benefits
Risks
Resource impact
Security impact
Migration impact
Decision
```

Do not silently drift from the architecture.

---

# 44. Final Vision

The intended final experience:

```text
Fresh Linux Server
       ↓
One-command Installer
       ↓
System Detection
       ↓
Compatibility Check
       ↓
Virtualization Choice
       ↓
Storage Choice
       ↓
Network Choice
       ↓
Public/Private Connectivity
       ↓
Security Configuration
       ↓
Safe Installation
       ↓
Health Check
       ↓
Mystic Hypervisor
       |
       +── VMs
       +── Containers
       +── Storage
       +── Networks
       +── Images
       +── Snapshots
       +── Monitoring
       +── Users/RBAC
       +── API
       +── CLI
       +── Audit
       +── Remote Hosts
       +── Private Gateway/Tunnel
       +── Backups
```

The product should feel like a lightweight appliance/control plane while remaining transparent, scriptable, secure, and based on real Linux virtualization primitives.

---

# 45. Current Baseline Decision

Unless explicitly changed later:

- Product name: **Mystic Hypervisor**
- Primary provider: **Incus**
- Container-only option: **LXC**
- Full VM option: **KVM/QEMU**
- General recommendation: **Incus + KVM**
- Backend direction: **Go**
- Frontend direction: **React + TypeScript**
- API: **REST + WebSocket**
- API version: **v1**
- CLI: **mysticctl**
- Installer: **single bootstrap + versioned installer**
- Default deployment: **single host**
- Future deployment: **multi-host**
- Private networking: **first-class**
- Public gateway/port forwarding: **supported**
- Outbound encrypted tunnel: **supported as optional architecture**
- Safety: **dry-run + backup/state + rollback + doctor + safe uninstall**
- Production data: **real only**
- Development: **local first, real VPS for controlled integration testing**
- Architecture: **provider abstraction**
- Core priority: **lightweight + safe + real infrastructure management**

This document is the project's baseline. Any deviation should be deliberate, documented, and approved before implementation.

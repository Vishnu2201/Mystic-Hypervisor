# Mystic Hypervisor — Security Policy & Model

**Status:** Milestone 5 — Real Incus Workload Provisioning & Provider Execution  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Security Architecture & Principles

Mystic Hypervisor is designed with a **Secure by Default** posture across control plane, API, and installer:

- **HTTPS / TLS**: All web and API traffic must use TLS 1.3/1.2. Self-signed certificates are generated automatically during installation if custom certificates are not specified.
- **Installer POSIX Shell Hygiene**:
  - No `eval` statements.
  - All variables properly quoted.
  - No blind `sudo` execution or unvalidated user inputs.
  - Transaction logs automatically redact sensitive key-value patterns (`password=`, `token=`, `secret=`).
- **No Lockout Protection**: The installer Network Safety Engine verifies active SSH sessions, SSH listening ports, and management routes before applying network changes to prevent administrator lockout.
- **Secret Redaction**: Automatic stripping and masking of secrets (passwords, tokens, private keys, session cookies) in application logs and installer transaction state files.

## 2. Real Incus Provisioning & Provider Security (Milestone 5)

- **Safe Command Executions**: All hypervisor operations (`incus list`, `incus launch`, `incus start`, etc.) execute via strict `exec.CommandContext` parameter arrays (no shell string concatenation or evaluation).
- **Explicit Approval Boundary**: Workload specifications and provisioning plans MUST be explicitly approved by an administrator before hypervisor creation commands are dispatched.
- **Resource Boundary Limits**: Memory, CPU, and storage limits are validated prior to plan generation to prevent resource exhaustion attacks on target hosts.
- **Zero Provider Assumption**: Non-Linux development environments safely report `interfaces.ErrProviderUnavailable` rather than falling back to unauthenticated or unsafe mock commands.

## 3. Network Exposure Security Policy (Milestone 3E / Milestone 4)

- **Default Unconfigured Posture (`UNCONFIGURED`)**: Mystic Hypervisor does NOT open public ports or enable WAN access automatically.
- **Explicit Exposure Scoping**: Administrators must explicitly choose between `HOST` control plane exposure and `WORKLOAD` guest exposure.
- **NAT Boundary Protection**: The hypervisor never assumes WAN port availability and alerts administrators before exposing direct public IP interfaces or setting up port forwarding.
- **Private Only Default Recommendation**: `PRIVATE_ONLY` mode (LAN/VPN) is recommended for hosts without dedicated firewall infrastructure.

## 4. Default Roles & Permissions

- `Administrator`: Full system control including user management, host configuration, provider selection, instance lifecycle, storage, and networking.
- `Operator`: Instance lifecycle management (start, stop, restart, snapshot, console), image viewing, and monitoring.
- `Viewer`: Read-only access to instance status, host metrics, and public network configuration.

## 5. Secret Protection Policy

1. **No Credentials in Source**: Code and repository assets must never contain hardcoded secrets, private keys, API tokens, or default passwords.
2. **Log & Transaction Redaction**: `backend/internal/logging` and `installer/modules/transaction.sh` sanitize all log writers and step metadata.
3. **Environment & Filesystem**: Sensitive configuration files stored under `/etc/mystic/` or `/var/lib/mystic/` must have file permissions `0600` or `0700` owned by `mystic:mystic`.



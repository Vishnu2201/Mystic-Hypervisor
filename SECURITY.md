# Mystic Hypervisor — Security Policy & Model

**Status:** Milestone 2 — Safe Installer Foundation  
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

## 2. Default Roles & Permissions

- `Administrator`: Full system control including user management, host configuration, provider selection, instance lifecycle, storage, and networking.
- `Operator`: Instance lifecycle management (start, stop, restart, snapshot, console), image viewing, and monitoring.
- `Viewer`: Read-only access to instance status, host metrics, and public network configuration.

## 3. Secret Protection Policy

1. **No Credentials in Source**: Code and repository assets must never contain hardcoded secrets, private keys, API tokens, or default passwords.
2. **Log & Transaction Redaction**: `backend/internal/logging` and `installer/modules/transaction.sh` sanitize all log writers and step metadata.
3. **Environment & Filesystem**: Sensitive configuration files stored under `/etc/mystic/` or `/var/lib/mystic/` must have file permissions `0600` or `0700` owned by `mystic:mystic`.

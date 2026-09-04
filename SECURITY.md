# Mystic Hypervisor — Security Policy & Model

**Status:** Milestone 1 — Engineering Foundation  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Security Architecture

Mystic Hypervisor is designed with a **Secure by Default** security posture:

- **TLS / HTTPS**: All web and API traffic must use TLS 1.3/1.2. Self-signed certificates are generated automatically on installation if Let's Encrypt or custom certificates are not configured.
- **Authentication**: JWT / Session-token based authentication with password hashing using Argon2id / bcrypt.
- **Role-Based Access Control (RBAC)**: Fine-grained permissions controlling host, instance, network, storage, user, and audit log access.
- **Audit Logging**: Mandatory structured logging for all privileged or state-changing operations.
- **Secret Redaction**: Automatic stripping and masking of secrets (passwords, tokens, private keys, session cookies) in application logs.
- **No Arbitrary Command Execution**: Mystic API never exposes arbitrary host shell execution endpoints.

## 2. Default Roles & Permissions

- `Administrator`: Full system control including user management, host configuration, provider selection, instance lifecycle, storage, and networking.
- `Operator`: Instance lifecycle management (start, stop, restart, snapshot, console), image viewing, and monitoring.
- `Viewer`: Read-only access to instance status, host metrics, and public network configuration.

### Example Permission Identifiers
- `host:read`, `host:manage`
- `instance:read`, `instance:create`, `instance:start`, `instance:stop`, `instance:delete`, `instance:console`
- `network:read`, `network:manage`
- `storage:read`, `storage:manage`
- `user:read`, `user:manage`
- `audit:read`

## 3. Secret Protection Policy

1. **No Credentials in Code**: Source code must never contain hardcoded secrets, private keys, API tokens, or default passwords.
2. **Log Redaction**: `backend/internal/logging` wraps all structured log writers with a secret redactor regex filter replacing sensitive patterns with `[REDACTED]`.
3. **Environment & Filesystem**: Sensitive configuration files stored under `/etc/mystic/` or `/var/lib/mystic/` must have file permissions `0600` or `0700` owned by `mystic:mystic`.

## 4. Reporting Vulnerabilities

If you identify a security vulnerability in Mystic Hypervisor, please report it privately to security@mysticpanel.dev or file a confidential report. Do not create public issues for security vulnerabilities.

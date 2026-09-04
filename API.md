# Mystic Hypervisor — REST API & WebSocket Specification

**Status:** Milestone 1 — Engineering Foundation  
**Base URL:** `/api/v1`  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## Overview

The Mystic REST API provides full programmatic control over host inspection, instance management, storage, networking, user RBAC, and audit logs. Real-time metrics and state changes are broadcast via WebSocket at `/api/v1/events`.

## Authentication

All requests (except `/api/v1/health` and `/api/v1/auth/login`) require a Bearer token header:
```http
Authorization: Bearer <mystic_token>
```

## Endpoints Overview

### System & Health
- `GET /api/v1/health` — System health and daemon status.
- `GET /api/v1/version` — Version information.
- `GET /api/v1/doctor` — Diagnostics results.

### Hosts
- `GET /api/v1/hosts` — List registered host nodes.
- `GET /api/v1/hosts/{id}` — Retrieve host specifications and real-time metrics.

### Instances (VMs & Containers)
- `GET /api/v1/instances` — List instances (combines DB metadata with live provider status).
- `POST /api/v1/instances` — Create a new instance (async: returns `TaskID`).
- `GET /api/v1/instances/{id}` — Get instance details.
- `PATCH /api/v1/instances/{id}` — Update instance metadata/configuration.
- `DELETE /api/v1/instances/{id}` — Delete instance.
- `POST /api/v1/instances/{id}/start` — Start instance.
- `POST /api/v1/instances/{id}/stop` — Stop instance.
- `POST /api/v1/instances/{id}/restart` — Restart instance.
- `GET /api/v1/instances/{id}/metrics` — Fetch live CPU/RAM/IO metrics.

### Provider Capabilities
- `GET /api/v1/providers` — List available virtualization providers and their capability matrices.

### Storage & Networking
- `GET /api/v1/storage` — List storage pools and volumes.
- `GET /api/v1/networks` — List bridges and network interfaces.

### Audit & Security
- `GET /api/v1/users` — Manage users and roles.
- `GET /api/v1/audit` — Query audit logs.

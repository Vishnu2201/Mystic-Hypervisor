# Mystic Hypervisor — Testing Guide & Policy

**Status:** Milestone 7 — Real VPS Integration & Controlled Incus Validation  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Testing Philosophy

1. **Empirical Verification**: Never claim a test passed unless it was actually executed.
2. **Platform Awareness**:
   - **WINDOWS LOCAL TESTS**: Tests flags, parsing, dry-run safety, plan rendering, JSON format (`detected` vs `configuration`), exposure modes, gateway fields, and dev-host safety guards.
   - **BACKEND UNIT TESTS (`backend/internal/workloads/manager_test.go`, `execution_test.go`, `allocator_test.go`, `incus_test.go`)**: Tests workload draft creation, spec validation, plan generation, explicit approval boundary enforcement (`ApprovePlan`), Incus provider preflight (`Preflight`), health status classification (`Installed`, `Reachable`, `Operational`, `Capable`), ownership classification (`MYSTIC_OWNED`, `EXTERNAL`), Incus provider unavailable handling (`interfaces.ErrProviderUnavailable`), reconciler state drift detection (`RUNNING` vs `STOPPED`), port allocation requests, Execution Guard idempotency (`OpKey`), delete safety (`user.mystic.owned`), plan immutability (`PlanHash`), state-aware lifecycle guards (`StartWorkload`, `StopWorkload`), context timeouts, outcome classification (`SUCCESS`, `FAILED`, `UNKNOWN`), and structured audit logging.
   - **LINUX TARGET TESTS**: Tests Linux kernel inspection, `/etc/os-release` parsing, `/dev/kvm` presence, package manager checks, real `incus` CLI query parsing, and network route inspection.
3. **No Fake Infrastructure Test Stubs**: Automated unit tests must use isolated test doubles/structures rather than polluting production paths with mock data.

## 2. Running Test Suites

### Installer Test Suite
```bash
bash installer/tests/test_installer.sh
```

### Automated Backend & CLI Unit Tests (Requires Go 1.22+)
```bash
cd backend && go test -v -cover ./...
cd cli && go test -v -cover ./...
```

### Frontend Type Check & Production Build Verification
```bash
cd frontend && npm run build
```

## 3. Controlled Real VPS Testing Procedure (Milestone 7 Manual Test)

Follow these phases strictly when performing the first controlled test on the target Linux VPS host:

### PHASE A — READ ONLY PREPARATION
1. Execute installer dry-run:
   ```bash
   ./installer/install.sh --dry-run
   ```
2. Build backend and frontend binaries on the target host or deploy compiled AMD64 artifacts:
   ```bash
   cd backend && go build -v -o bin/mysticd ./cmd/mysticd
   cd ../frontend && npm run build
   ```
3. Start `mysticd` manually in foreground (do NOT register as a systemd service yet).

### PHASE B — DISCOVERY
1. Query provider preflight endpoint:
   ```bash
   curl -s http://127.0.0.1:8080/api/v1/providers/incus/preflight
   ```
2. Verify output confirms:
   - `health_status.reachable` = `true`
   - Existing instances discovered with correct ownership classification (`MYSTIC_OWNED` vs `EXTERNAL`).
   - Discovered networks (e.g. `incusbr0`), storage pools (`default`), and images are reported.
   - ZERO mutations have occurred on the host.

### PHASE C — ADMINISTRATOR REVIEW
1. Inspect discovered infrastructure in Mystic Web UI or CLI.
2. Confirm that pre-existing host workloads (e.g., Docker, Pterodactyl, existing Incus VMs) are intact and untouched.

### PHASE D — CONTROLLED TEST WORKLOAD CREATION (EXPLICIT APPROVAL ONLY)
1. Create ONE temporary integration-test workload via UI or API:
   - **Name**: `mystic-integration-test-01`
   - **Network**: Discovered `incusbr0`
   - **Storage Pool**: Discovered `default`
   - **Exposure Mode**: `PRIVATE_ONLY` (no WAN ports, no NAT modifications)
   - **Limits**: 1 CPU core, 512MB RAM, 5GB storage
2. Generate plan (`POST /api/v1/workloads/{id}/plan`).
3. Explicitly approve plan (`POST /api/v1/workloads/{id}/approve`).
4. Execute provisioning (`POST /api/v1/workloads/{id}/provision`).

### PHASE E — VERIFICATION
1. Verify instance creation on Incus directly:
   ```bash
   incus list
   ```
2. Compare live state reported by `incus list` with Mystic UI (`Status: RUNNING`, `Sync: IN_SYNC`).

### PHASE F — LIFECYCLE TESTING
1. Test controlled lifecycle transitions:
   - `StopWorkload` -> verify `incus list` reports `STOPPED`.
   - `StartWorkload` -> verify `incus list` reports `RUNNING`.
   - `ReconcileWorkload` -> verify state sync.

### PHASE G — CLEANUP & DELETION
1. Delete ONLY the Mystic-owned test workload:
   - `DELETE /api/v1/workloads/{id}`
2. Verify via `incus list` that `mystic-integration-test-01` was removed while all pre-existing host workloads remain completely untouched.






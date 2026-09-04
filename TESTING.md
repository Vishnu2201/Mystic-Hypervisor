# Mystic Hypervisor — Testing Guide & Policy

**Status:** Milestone 5 — Real Incus Workload Provisioning & Provider Execution  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Testing Philosophy

1. **Empirical Verification**: Never claim a test passed unless it was actually executed.
2. **Platform Awareness**:
   - **WINDOWS LOCAL TESTS**: Tests flags, parsing, dry-run safety, plan rendering, JSON format (`detected` vs `configuration`), exposure modes, gateway fields, and dev-host safety guards.
   - **BACKEND UNIT TESTS (`backend/internal/workloads/manager_test.go`, `allocator_test.go`)**: Tests workload draft creation, spec validation, plan generation, explicit approval boundary enforcement (`ApprovePlan`), Incus provider unavailable handling (`interfaces.ErrProviderUnavailable`), reconciler state drift detection (`RUNNING` vs `STOPPED`), port allocation requests, bounds checking, range size matching, reversed ranges, duplicate rules, TCP/UDP collisions, SSH management port 22 reservations, auto pool allocation (`ALLOCATION_POOL_UNCONFIGURED`), and host listener collisions.
   - **LINUX TARGET TESTS**: Tests Linux kernel inspection, `/etc/os-release` parsing, `/dev/kvm` presence, package manager checks, real `incus` CLI query parsing, and network route inspection.
3. **No Fake Infrastructure Test Stubs**: Automated unit tests must use isolated test structures rather than polluting production paths with mock data.

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




# Mystic Hypervisor — Testing Guide & Policy

**Status:** Milestone 2 — Safe Installer Foundation  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## 1. Testing Philosophy

1. **Empirical Verification**: Never claim a test passed unless it was actually executed.
2. **Platform Awareness**:
   - **WINDOWS LOCAL TESTS**: Tests flags, parsing, dry-run safety, plan rendering, JSON format, and dev-host safety guards.
   - **LINUX TARGET TESTS**: Tests Linux kernel inspection, `/etc/os-release` parsing, `/dev/kvm` presence, package manager checks, and network route inspection.
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

### Frontend Type Check & Verification
```bash
cd frontend && npm run build
```

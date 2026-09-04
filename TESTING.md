# Mystic Hypervisor — Testing Guide & Policy

**Status:** Milestone 1 — Engineering Foundation  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## Testing Philosophy

1. **Empirical Verification**: Never claim a test passed unless it was actually executed.
2. **Isolated Unit Testing**: Unit tests exercise code contracts (configuration loading, log redaction, capability matrix checks, state reconciler logic) without altering host system configuration or attempting connection to real external hypervisors.
3. **No Fake Infrastructure Test Stubs**: Automated unit tests must use isolated test structures rather than polluting production paths with mock data.

## Running Tests

### Automated Backend Tests
```bash
cd backend
go test -v -cover ./...
```

### Automated CLI Tests
```bash
cd cli
go test -v -cover ./...
```

### Frontend Type Check & Verification
```bash
cd frontend
npm run build
```

### Installer Dry Run Test
```bash
bash installer/install.sh --dry-run
```

## Milestone 1 Test Coverage Requirements

- **Configuration**: Verification of default values, environment variable overrides, and secret redactor logic.
- **Logging**: Verification of secret masking across all log levels.
- **Provider Abstraction**: Verification of capability matrix checks, provider registration, and error propagation (`ErrUnsupportedOperation`).
- **State Reconciliation**: Verification that provider runtime state overrides database metadata when resolving active instance state.
- **CLI Subcommands**: Verification of subcommand flag parsing and reporting of un-implemented feature stubs.

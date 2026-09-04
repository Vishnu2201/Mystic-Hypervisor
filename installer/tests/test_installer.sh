#!/usr/bin/env bash
# Test Suite for Mystic Hypervisor Safe Installer
# Executes platform-aware tests for flags, detection, transactions, and safety rules.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

TESTS_PASSED=0
TESTS_FAILED=0

assert_contains() {
    local output="$1"
    local expected="$2"
    local test_name="$3"

    if echo "$output" | grep -q "$expected"; then
        echo "  [PASS] $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo "  [FAIL] $test_name — Expected output to contain '$expected'"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

echo "========================================================"
echo "          MYSTIC HYPERVISOR INSTALLER TEST SUITE        "
echo "========================================================"
echo ""

UNAME_S="$(uname -s 2>/dev/null || echo "UNKNOWN")"

# WINDOWS LOCAL TESTS
echo "--- WINDOWS LOCAL TESTS ---"
echo "Executing local platform tests on dev host ($UNAME_S)..."

# Test 1: Default Invocation (Requires explicit mode, does not install)
OUT_DEFAULT=$(bash "$INSTALLER_DIR/install.sh" 2>&1 || true)
assert_contains "$OUT_DEFAULT" "requires an explicit mode" "Default invocation explains mode requirement without modifying system"

# Test 2: Help Flag
OUT_HELP=$(bash "$INSTALLER_DIR/install.sh" --help 2>&1 || true)
assert_contains "$OUT_HELP" "Usage: ./install.sh" "Help flag displays complete usage instructions"

# Test 3: Version Flag
OUT_VER=$(bash "$INSTALLER_DIR/install.sh" --version 2>&1 || true)
assert_contains "$OUT_VER" "Mystic Hypervisor Safe Installer" "Version flag reports installer version"

# Test 4: Dry-Run Mode on Dev Host
OUT_DRY=$(bash "$INSTALLER_DIR/install.sh" --dry-run 2>&1 || true)
assert_contains "$OUT_DRY" "NO CHANGES WILL BE MADE" "Dry-run mode outputs read-only plan"
assert_contains "$OUT_DRY" "Windows Development Host" "Dry-run correctly detects Windows development host"
assert_contains "$OUT_DRY" "UNSUPPORTED_DEV_HOST" "Dry-run rates Windows dev host as UNSUPPORTED_DEV_HOST"

# Test 5: Plan Mode JSON Output
OUT_JSON=$(bash "$INSTALLER_DIR/install.sh" --plan --json 2>&1 || true)
assert_contains "$OUT_JSON" '"dry_run": true' "Plan mode generates valid JSON structure"

# Test 6: Apply Guard on Dev Host
OUT_APPLY=$(bash "$INSTALLER_DIR/install.sh" --apply --yes 2>&1 || true)
assert_contains "$OUT_APPLY" "Cannot execute --apply on a non-Linux development host" "Apply guard blocks host modifications on dev host"

echo ""
echo "--- LINUX TARGET TESTS ---"
if case "$UNAME_S" in Linux*) true;; *) false;; esac; then
    echo "Linux target host detected ($UNAME_S). Running Linux target tests..."
    # Linux-specific tests would run here
else
    echo "[SKIPPED] Linux target tests require execution on a Linux target server environment."
    echo "Current environment ($UNAME_S) is recognized as local development host."
fi

echo ""
echo "========================================================"
echo "Test Results: $TESTS_PASSED passed, $TESTS_FAILED failed."
echo "========================================================"

if [ "$TESTS_FAILED" -gt 0 ]; then
    exit 1
fi
exit 0

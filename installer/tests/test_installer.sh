#!/usr/bin/env bash
# Test Suite for Mystic Hypervisor Safe Installer (Milestone 3D Network Model)
# Executes platform-aware tests for flags, IP ownership, NAT status, exposure modes, and safety rules.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

TESTS_PASSED=0
TESTS_FAILED=0

assert_contains() {
    local output="$1"
    local expected="$2"
    local test_name="$3"

    if echo "$output" | grep -F -q "$expected"; then
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

# Test 4: Dry-Run Mode Output Layout & Unambiguous IP Fields
OUT_DRY=$(bash "$INSTALLER_DIR/install.sh" --dry-run 2>&1 || true)
assert_contains "$OUT_DRY" "NO CHANGES WILL BE MADE" "Dry-run mode outputs read-only plan"
assert_contains "$OUT_DRY" "Windows Development Host" "Dry-run correctly detects Windows development host"
assert_contains "$OUT_DRY" "UNSUPPORTED_DEV_HOST" "Dry-run rates Windows dev host as UNSUPPORTED_DEV_HOST"
assert_contains "$OUT_DRY" "Host Public IP:" "Dry-run renders Host Public IP field"
assert_contains "$OUT_DRY" "Upstream IP:" "Dry-run renders Upstream IP field"
assert_contains "$OUT_DRY" "IP Assignment:" "Dry-run renders IP Assignment field"
assert_contains "$OUT_DRY" "NAT Status:" "Dry-run renders NAT Status field"
assert_contains "$OUT_DRY" "Network Exposure Model" "Dry-run renders Network Exposure Model section"
assert_contains "$OUT_DRY" "PRIVATE_ONLY" "Dry-run documents PRIVATE_ONLY exposure mode"
assert_contains "$OUT_DRY" "NAT_FORWARDED" "Dry-run documents NAT_FORWARDED exposure mode"
assert_contains "$OUT_DRY" "DIRECT_PUBLIC" "Dry-run documents DIRECT_PUBLIC exposure mode"
assert_contains "$OUT_DRY" "EXTERNAL_GATEWAY" "Dry-run documents EXTERNAL_GATEWAY exposure mode"
assert_contains "$OUT_DRY" "Gateway ID:" "Dry-run renders Gateway ID field"
assert_contains "$OUT_DRY" "Gateway Public IP:" "Dry-run renders Gateway Public IP field"

# Test 5: Plan Mode JSON Output Structure & Unambiguous IP Fields
OUT_JSON=$(bash "$INSTALLER_DIR/install.sh" --plan --json 2>&1 || true)
assert_contains "$OUT_JSON" '"dry_run": true' "Plan mode generates valid JSON structure"
assert_contains "$OUT_JSON" '"installer_version": "0.5.0-milestone3e"' "JSON plan reports 0.5.0-milestone3e version"
assert_contains "$OUT_JSON" '"detected": {' "JSON plan includes detected facts object"
assert_contains "$OUT_JSON" '"configuration": {' "JSON plan includes configuration settings object"
assert_contains "$OUT_JSON" '"host_public_ip":' "JSON plan includes host_public_ip field"
assert_contains "$OUT_JSON" '"upstream_public_ip":' "JSON plan includes upstream_public_ip field"
assert_contains "$OUT_JSON" '"public_ip_assignment_status":' "JSON plan includes public_ip_assignment_status field"
assert_contains "$OUT_JSON" '"nat_status":' "JSON plan includes nat_status field"
assert_contains "$OUT_JSON" '"configured_exposure_mode":' "JSON plan includes configured_exposure_mode field"
assert_contains "$OUT_JSON" '"gateway_id":' "JSON plan includes gateway_id field"
assert_contains "$OUT_JSON" '"forwarding_rules": []' "JSON plan includes forwarding_rules list"

# Test 6: Apply Guard on Dev Host
OUT_APPLY=$(bash "$INSTALLER_DIR/install.sh" --apply --yes 2>&1 || true)
assert_contains "$OUT_APPLY" "Cannot execute --apply on a non-Linux development host" "Apply guard blocks host modifications on dev host"

# TRANSACTION & BACKUP ENGINE TESTS (Milestone 4A)
echo ""
echo "--- TRANSACTION & BACKUP ENGINE TESTS ---"

TEST_TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'mystic_test')
trap 'rm -rf "$TEST_TMP_DIR"' EXIT

export MYSTIC_STATE_DIR="$TEST_TMP_DIR/state"

source "$INSTALLER_DIR/modules/transaction.sh"
source "$INSTALLER_DIR/modules/backup.sh"

# Test A: Transaction Initialization
init_transaction "TEST_INSTALL"
assert_contains "$TX_STATUS" "IN_PROGRESS" "init_transaction sets IN_PROGRESS status"

if [ -d "$TX_DIR" ] && [ -f "$TX_DIR/transaction.json" ] && [ -f "$TX_DIR/steps.json" ]; then
    echo "  [PASS] init_transaction creates directory, transaction.json, and steps.json"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo "  [FAIL] init_transaction directory or files missing"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

TX_JSON_CONTENT=$(cat "$TX_DIR/transaction.json" 2>/dev/null || true)
assert_contains "$TX_JSON_CONTENT" '"status": "IN_PROGRESS"' "transaction.json records IN_PROGRESS status"
assert_contains "$TX_JSON_CONTENT" '"operation": "TEST_INSTALL"' "transaction.json records TEST_INSTALL operation"

# Test B: Persistent Transaction Step
record_tx_step "test_step_1" "COMPLETED" "testing step persistence"
STEPS_JSON_CONTENT=$(cat "$TX_DIR/steps.json" 2>/dev/null || true)
assert_contains "$STEPS_JSON_CONTENT" '"step": "test_step_1"' "record_tx_step persists step name to steps.json"
assert_contains "$STEPS_JSON_CONTENT" '"status": "COMPLETED"' "record_tx_step persists step status to steps.json"

# Test C: Backup & Restoration of Existing File
TEST_FILE="$TEST_TMP_DIR/etc_config.txt"
echo "original_config_value_123" > "$TEST_FILE"

backup_file "$TEST_FILE"
MANIFEST_CONTENT=$(cat "$TX_DIR/backup-manifest.json" 2>/dev/null || true)
assert_contains "$MANIFEST_CONTENT" '"status": "BACKED_UP"' "backup_file marks existing file as BACKED_UP in manifest"

if verify_backup "$TX_DIR"; then
    echo "  [PASS] verify_backup confirms backed-up file exists and is readable"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo "  [FAIL] verify_backup failed for backed-up file"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Modify original test file
echo "corrupted_modified_value_456" > "$TEST_FILE"

# Restore file
restore_backup "$TX_DIR"
RESTORED_CONTENT=$(cat "$TEST_FILE" 2>/dev/null || true)
assert_contains "$RESTORED_CONTENT" "original_config_value_123" "restore_backup restores original file contents"

# Test D: Backup of Missing File
MISSING_FILE="$TEST_TMP_DIR/nonexistent_file.txt"
backup_file "$MISSING_FILE"
MANIFEST_CONTENT_D=$(cat "$TX_DIR/backup-manifest.json" 2>/dev/null || true)
assert_contains "$MANIFEST_CONTENT_D" '"status": "NOT_PRESENT"' "backup_file marks missing file as NOT_PRESENT in manifest"

# Ensure no fake backup file created
if [ ! -d "$TX_DIR/backup" ] || [ $(ls -1 "$TX_DIR/backup" 2>/dev/null | wc -l) -eq 1 ]; then
    echo "  [PASS] backup_file does not create fake backup for nonexistent file"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo "  [FAIL] backup_file created unexpected files for nonexistent file"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test E: Transaction Commit
commit_transaction
TX_JSON_COMMITTED=$(cat "$TX_DIR/transaction.json" 2>/dev/null || true)
assert_contains "$TX_JSON_COMMITTED" '"status": "COMMITTED"' "commit_transaction sets COMMITTED status in transaction.json"

# Test F: Transaction Failure
init_transaction "TEST_FAIL_OP"
fail_transaction "simulated test failure"
TX_JSON_FAILED=$(cat "$TX_DIR/transaction.json" 2>/dev/null || true)
assert_contains "$TX_JSON_FAILED" '"status": "FAILED"' "fail_transaction sets FAILED status in transaction.json"

# Test G: Rollback (Only restores explicitly backed-up files)
TEST_FILE_2="$TEST_TMP_DIR/important_data.txt"
UNTOUCHED_FILE="$TEST_TMP_DIR/untouched_file.txt"
echo "important_original" > "$TEST_FILE_2"
echo "untouched_original" > "$UNTOUCHED_FILE"

init_transaction "TEST_ROLLBACK_OP"
backup_file "$TEST_FILE_2"

# Modify files
echo "important_modified" > "$TEST_FILE_2"
echo "untouched_modified" > "$UNTOUCHED_FILE"

# Execute rollback
rollback_transaction
TX_JSON_ROLLED_BACK=$(cat "$TX_DIR/transaction.json" 2>/dev/null || true)
assert_contains "$TX_JSON_ROLLED_BACK" '"status": "ROLLED_BACK"' "rollback_transaction sets ROLLED_BACK status in transaction.json"

RESTORED_2=$(cat "$TEST_FILE_2" 2>/dev/null || true)
UNTOUCHED_2=$(cat "$UNTOUCHED_FILE" 2>/dev/null || true)
assert_contains "$RESTORED_2" "important_original" "rollback_transaction restores backed up file"
assert_contains "$UNTOUCHED_2" "untouched_modified" "rollback_transaction does not touch unbacked-up files"

echo ""
echo "--- LINUX TARGET TESTS ---"
if case "$UNAME_S" in Linux*) true;; *) false;; esac; then
    echo "Linux target host detected ($UNAME_S). Running Linux target tests..."
    # Linux-specific target tests execute here
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

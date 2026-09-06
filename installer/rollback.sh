#!/usr/bin/env bash
# Mystic Hypervisor Rollback Controller
# Restores system state using persistent transaction backups.
#
# ARCHITECTURAL LIFECYCLE BOUNDARIES:
# - detection: System discovery, OS identification & capability inspection
# - planning: Installation spec creation, network exposure validation & dry-run planning
# - transaction state: Persistent transaction logging, audit history & state machine
# - backup: Pre-change file backups, verification & restoration (USED HERE)
# - future mutation: (Milestone 4B/4C) Host system package installation, binary placement & systemd service setup

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -f "$SCRIPT_DIR/modules/transaction.sh" ]; then
    source "$SCRIPT_DIR/modules/transaction.sh"
    source "$SCRIPT_DIR/modules/backup.sh"
fi

TARGET_TX_ID="${1:-}"

if [ -z "$TARGET_TX_ID" ]; then
    STATE_DIR="${MYSTIC_STATE_DIR:-/var/lib/mystic}/transactions"
    if [ -d "$STATE_DIR" ]; then
        TARGET_TX_ID=$(ls -1t "$STATE_DIR" 2>/dev/null | grep -E '^TX_' | head -n 1 || true)
    fi
fi

if [ -z "$TARGET_TX_ID" ]; then
    echo "Error: No transaction ID provided and no prior recorded transaction found." >&2
    exit 1
fi

echo "=== Mystic Hypervisor Rollback Controller ==="
echo "Target Transaction ID: $TARGET_TX_ID"

if rollback_transaction "$TARGET_TX_ID"; then
    echo "Rollback completed successfully for transaction '$TARGET_TX_ID'."
    exit 0
else
    echo "Error: Rollback failed for transaction '$TARGET_TX_ID'." >&2
    exit 1
fi

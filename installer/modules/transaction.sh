#!/usr/bin/env bash
# Transaction Model & Persistent Logging Module for Mystic Hypervisor Installer
#
# ARCHITECTURAL LIFECYCLE BOUNDARIES:
# - detection: System discovery, OS identification & capability inspection
# - planning: Installation spec creation, network exposure validation & dry-run planning
# - transaction state: Persistent transaction logging, audit history & state machine (THIS MODULE)
# - backup: Pre-change file backups, verification & restoration
# - future mutation: (Milestone 4B/4C) Host system package installation, binary placement & systemd service setup

TX_ID=""
TX_OPERATION=""
TX_STATUS="NOT_STARTED"
TX_STARTED_AT=""
TX_COMPLETED_AT=""
TX_STEPS=()
TX_LOG_DIR=""
TX_DIR=""
TX_BACKUP_DIR=""

_get_utc_timestamp() {
    date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u 2>/dev/null || echo "1970-01-01T00:00:00Z"
}

_save_transaction_json() {
    if [ -z "${TX_DIR:-}" ] || [ ! -d "$TX_DIR" ]; then
        return 0
    fi
    local tx_file="${TX_DIR}/transaction.json"
    
    if command -v jq >/dev/null 2>&1; then
        jq -n \
            --arg tx_id "${TX_ID:-}" \
            --arg op "${TX_OPERATION:-}" \
            --arg status "${TX_STATUS:-}" \
            --arg started_at "${TX_STARTED_AT:-}" \
            --arg completed_at "${TX_COMPLETED_AT:-}" \
            --arg tx_dir "${TX_DIR:-}" \
            --arg backup_dir "${TX_BACKUP_DIR:-}" \
            '{
                transaction_id: $tx_id,
                operation: $op,
                status: $status,
                started_at: $started_at,
                completed_at: (if $completed_at == "" then null else $completed_at end),
                transaction_dir: $tx_dir,
                backup_dir: $backup_dir
            }' > "$tx_file.tmp" && mv "$tx_file.tmp" "$tx_file"
    elif command -v python3 >/dev/null 2>&1; then
        python3 -c 'import json, sys, os
tx_file, tx_id, op, status, started, completed, tx_dir, backup_dir = sys.argv[1:9]
data = {
    "transaction_id": tx_id,
    "operation": op,
    "status": status,
    "started_at": started,
    "completed_at": completed if completed else None,
    "transaction_dir": tx_dir,
    "backup_dir": backup_dir
}
with open(tx_file + ".tmp", "w") as f:
    json.dump(data, f, indent=2)
os.replace(tx_file + ".tmp", tx_file)
' "$tx_file" "${TX_ID:-}" "${TX_OPERATION:-}" "${TX_STATUS:-}" "${TX_STARTED_AT:-}" "${TX_COMPLETED_AT:-}" "${TX_DIR:-}" "${TX_BACKUP_DIR:-}"
    else
        local comp_val="null"
        if [ -n "${TX_COMPLETED_AT:-}" ]; then
            comp_val="\"$TX_COMPLETED_AT\""
        fi
        cat <<EOF > "$tx_file.tmp"
{
  "transaction_id": "${TX_ID:-}",
  "operation": "${TX_OPERATION:-}",
  "status": "${TX_STATUS:-}",
  "started_at": "${TX_STARTED_AT:-}",
  "completed_at": $comp_val,
  "transaction_dir": "${TX_DIR:-}",
  "backup_dir": "${TX_BACKUP_DIR:-}"
}
EOF
        mv "$tx_file.tmp" "$tx_file"
    fi
}

_save_steps_json() {
    if [ -z "${TX_DIR:-}" ] || [ ! -d "$TX_DIR" ]; then
        return 0
    fi
    local steps_file="${TX_DIR}/steps.json"
    
    if command -v jq >/dev/null 2>&1; then
        local json_arr="[]"
        if [ "${#TX_STEPS[@]}" -gt 0 ]; then
            local raw_items=""
            for step_json in "${TX_STEPS[@]}"; do
                if [ -n "$raw_items" ]; then
                    raw_items+=",${step_json}"
                else
                    raw_items="${step_json}"
                fi
            done
            json_arr="[${raw_items}]"
        fi
        jq -n --argjson steps "$json_arr" '{steps: $steps}' > "$steps_file.tmp" && mv "$steps_file.tmp" "$steps_file"
    elif command -v python3 >/dev/null 2>&1; then
        python3 -c 'import json, sys, os
steps_file = sys.argv[1]
raw_steps = sys.argv[2]
items = []
if raw_steps:
    for line in raw_steps.split("|||"):
        line = line.strip()
        if line:
            try:
                items.append(json.loads(line))
            except Exception:
                pass
with open(steps_file + ".tmp", "w") as f:
    json.dump({"steps": items}, f, indent=2)
os.replace(steps_file + ".tmp", steps_file)
' "$steps_file" "$(IFS='|||'; echo "${TX_STEPS[*]}")"
    else
        local json_body="{\n  \"steps\": [\n"
        local first=1
        for step_json in "${TX_STEPS[@]}"; do
            if [ $first -eq 1 ]; then
                first=0
            else
                json_body+=",\n"
            fi
            json_body+="    $step_json"
        done
        json_body+="\n  ]\n}"
        printf "%b" "$json_body" > "$steps_file.tmp" && mv "$steps_file.tmp" "$steps_file"
    fi
}

init_transaction() {
    local op="$1"
    local timestamp
    timestamp=$(date -u +"%Y%m%d_%H%M%S" 2>/dev/null || echo "00000000_000000")
    local rand_suffix
    rand_suffix=$((RANDOM % 89999 + 10000))
    
    TX_ID="TX_${timestamp}_${rand_suffix}"
    TX_OPERATION="$op"
    TX_STATUS="IN_PROGRESS"
    TX_STARTED_AT=$(_get_utc_timestamp)
    TX_COMPLETED_AT=""
    TX_STEPS=()
    
    local base_state_dir="${MYSTIC_STATE_DIR:-/var/lib/mystic}"
    TX_LOG_DIR="${base_state_dir}/transactions"
    TX_DIR="${TX_LOG_DIR}/${TX_ID}"
    TX_BACKUP_DIR="${TX_DIR}/backup"
    
    # Attempt to create transaction and backup directory safely
    if ! mkdir -p "$TX_DIR" "$TX_BACKUP_DIR" 2>/dev/null; then
        if [ "$op" = "dry-run" ] || [ "$op" = "plan" ]; then
            # Read-only inspection modes do not force state directory creation if unprivileged
            TX_DIR=""
            TX_BACKUP_DIR=""
            return 0
        fi
        # Fallback for unprivileged testing when default /var/lib/mystic is not writable
        if [ -z "${MYSTIC_STATE_DIR:-}" ] && [ "${EUID:-$(id -u 2>/dev/null || echo 1000)}" -ne 0 ]; then
            base_state_dir="/tmp/mystic/state"
            TX_LOG_DIR="${base_state_dir}/transactions"
            TX_DIR="${TX_LOG_DIR}/${TX_ID}"
            TX_BACKUP_DIR="${TX_DIR}/backup"
            mkdir -p "$TX_DIR" "$TX_BACKUP_DIR" 2>/dev/null || true
        fi
        if [ -z "$TX_DIR" ] || [ ! -d "$TX_DIR" ]; then
            echo "Error: Failed to create transaction directory '${TX_DIR:-/var/lib/mystic}'. Aborting transaction." >&2
            return 1
        fi
    fi
    
    # Restrict permissions
    chmod 700 "$TX_DIR" 2>/dev/null || true
    chmod 700 "$TX_BACKUP_DIR" 2>/dev/null || true
    
    # Immediately persist transaction metadata and initial empty steps
    _save_transaction_json || return 1
    _save_steps_json || return 1
    
    return 0
}

record_tx_step() {
    local step_name="$1"
    local step_status="$2"
    local step_meta="${3:-none}"
    local step_time
    step_time=$(_get_utc_timestamp)
    
    # Redact sensitive patterns if any
    step_meta=$(echo "$step_meta" | sed -E 's/(password|secret|token|key)=[^ ]*/\1=[REDACTED]/g')
    
    # Safely escape string fields for JSON entry
    local meta_json
    if command -v jq >/dev/null 2>&1; then
        meta_json=$(jq -R . <<<"$step_meta")
        local name_json
        name_json=$(jq -R . <<<"$step_name")
        local status_json
        status_json=$(jq -R . <<<"$step_status")
        local time_json
        time_json=$(jq -R . <<<"$step_time")
        TX_STEPS+=("{\"step\":${name_json},\"status\":${status_json},\"timestamp\":${time_json},\"metadata\":${meta_json}}")
    else
        local escaped_meta="${step_meta//\\/\\\\}"
        escaped_meta="${escaped_meta//\"/\\\"}"
        TX_STEPS+=("{\"step\":\"$step_name\",\"status\":\"$step_status\",\"timestamp\":\"$step_time\",\"metadata\":\"$escaped_meta\"}")
    fi
    
    _save_steps_json
}

set_transaction_status() {
    local status="$1"
    case "$status" in
        IN_PROGRESS|COMMITTED|FAILED|ROLLED_BACK)
            TX_STATUS="$status"
            ;;
        *)
            echo "Error: Unsupported transaction status '$status'" >&2
            return 1
            ;;
    esac
    _save_transaction_json
}

commit_transaction() {
    TX_STATUS="COMMITTED"
    TX_COMPLETED_AT=$(_get_utc_timestamp)
    record_tx_step "commit_transaction" "COMMITTED" "Transaction committed successfully"
    _save_transaction_json
}

fail_transaction() {
    local reason="${1:-Unspecified error}"
    TX_STATUS="FAILED"
    TX_COMPLETED_AT=$(_get_utc_timestamp)
    record_tx_step "fail_transaction" "FAILED" "$reason"
    _save_transaction_json
}

rollback_transaction() {
    local target_tx_id="${1:-${TX_ID:-}}"
    local target_tx_dir=""
    
    if [ -n "$target_tx_id" ] && [ -d "${MYSTIC_STATE_DIR:-/var/lib/mystic}/transactions/${target_tx_id}" ]; then
        target_tx_dir="${MYSTIC_STATE_DIR:-/var/lib/mystic}/transactions/${target_tx_id}"
    elif [ -n "${TX_DIR:-}" ] && [ -d "$TX_DIR" ]; then
        target_tx_dir="$TX_DIR"
    fi
    
    if [ -z "$target_tx_dir" ] || [ ! -d "$target_tx_dir" ]; then
        echo "Error: Cannot perform rollback. Transaction directory not found for ID '${target_tx_id}'." >&2
        return 1
    fi
    
    local old_tx_dir="${TX_DIR:-}"
    TX_DIR="$target_tx_dir"
    
    record_tx_step "rollback_transaction" "IN_PROGRESS" "Restoring backed-up files from $target_tx_dir"
    
    if restore_backup "$target_tx_dir"; then
        TX_STATUS="ROLLED_BACK"
        TX_COMPLETED_AT=$(_get_utc_timestamp)
        record_tx_step "rollback_transaction" "ROLLED_BACK" "Rollback completed successfully"
        _save_transaction_json
        return 0
    else
        TX_STATUS="FAILED"
        TX_COMPLETED_AT=$(_get_utc_timestamp)
        record_tx_step "rollback_transaction" "FAILED" "Failed to restore backup files during rollback"
        _save_transaction_json
        return 1
    fi
}

get_transaction_summary() {
    echo "Transaction ID:        ${TX_ID:-NONE}"
    echo "Operation:             ${TX_OPERATION:-NONE}"
    echo "Status:                ${TX_STATUS:-NOT_STARTED}"
    echo "Started At:            ${TX_STARTED_AT:-NONE}"
    echo "Completed At:          ${TX_COMPLETED_AT:-NONE}"
    echo "Transaction Dir:       ${TX_DIR:-NONE}"
    echo "Recorded Steps:        ${#TX_STEPS[@]}"
}

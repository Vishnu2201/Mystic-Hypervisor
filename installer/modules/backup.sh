#!/usr/bin/env bash
# Backup & Safe Restoration Engine for Mystic Hypervisor Installer
#
# ARCHITECTURAL LIFECYCLE BOUNDARIES:
# - detection: System discovery, OS identification & capability inspection
# - planning: Installation spec creation, network exposure validation & dry-run planning
# - transaction state: Persistent transaction logging, audit history & state machine
# - backup: Pre-change file backups, verification & restoration (THIS MODULE)
# - future mutation: (Milestone 4B/4C) Host system package installation, binary placement & systemd service setup

_to_posix_path() {
    local p="$1"
    p="${p//\\//}"
    p="${p%$'\r'}"
    if [[ "$p" =~ ^[a-zA-Z]:/ ]]; then
        local drive
        drive=$(echo "${p:0:1}" | tr '[:upper:]' '[:lower:]')
        p="/${drive}${p:2}"
    fi
    echo "$p"
}

_init_backup_manifest() {
    local target_dir="${1:-${TX_DIR:-}}"
    if [ -z "$target_dir" ] || [ ! -d "$target_dir" ]; then
        return 1
    fi
    local manifest_file="${target_dir}/backup-manifest.json"
    if [ ! -f "$manifest_file" ]; then
        echo '{"files":[]}' > "$manifest_file"
    fi
}

_add_to_backup_manifest() {
    local src="$1"
    local dest="$2"
    local status="$3"
    local target_dir="${4:-${TX_DIR:-}}"
    local manifest_file="${target_dir}/backup-manifest.json"
    
    src=$(_to_posix_path "$src")
    if [ "$dest" != "null" ] && [ -n "$dest" ]; then
        dest=$(_to_posix_path "$dest")
    fi
    
    _init_backup_manifest "$target_dir" || return 1
    
    if command -v jq >/dev/null 2>&1; then
        jq --arg s "$src" --arg b "$dest" --arg st "$status" \
           '.files += [{"source": $s, "backup": (if $b == "null" or $b == "" then null else $b end), "status": $st}]' \
           "$manifest_file" > "$manifest_file.tmp" && mv "$manifest_file.tmp" "$manifest_file"
    elif command -v python3 >/dev/null 2>&1; then
        python3 -c 'import json, sys, os
manifest, src, dest, st = sys.argv[1:5]
backup_val = None if dest in ("null", "") else dest
try:
    with open(manifest, "r") as f:
        data = json.load(f)
except Exception:
    data = {"files": []}
if "files" not in data or not isinstance(data["files"], list):
    data["files"] = []
data["files"].append({"source": src, "backup": backup_val, "status": st})
with open(manifest + ".tmp", "w") as f:
    json.dump(data, f, indent=2)
os.replace(manifest + ".tmp", manifest)
' "$manifest_file" "$src" "$dest" "$status"
    else
        local dest_json="\"$dest\""
        if [ "$dest" = "null" ] || [ -z "$dest" ]; then
            dest_json="null"
        fi
        cat <<EOF > "$manifest_file.tmp"
{
  "files": [
    {
      "source": "$src",
      "backup": $dest_json,
      "status": "$status"
    }
  ]
}
EOF
        mv "$manifest_file.tmp" "$manifest_file"
    fi
}

prepare_backup_plan() {
    BACKUP_DIR="${TX_BACKUP_DIR:-${MYSTIC_STATE_DIR:-/var/lib/mystic}/backups/${TX_ID:-TX_DRYRUN}}"
    
    BACKUP_MANIFEST=()
    BACKUP_MANIFEST+=("system-info.json (Detected OS, Kernel, CPU, Memory)")
    BACKUP_MANIFEST+=("network-before.json (Preserves management routes & interface config)")
    BACKUP_MANIFEST+=("firewall-before.json (Preserves existing iptables/nftables rules)")
    BACKUP_MANIFEST+=("services-before.json (Records running systemd services)")
    BACKUP_MANIFEST+=("sshd-before.json (Preserves SSH daemon port & auth settings)")
    BACKUP_MANIFEST+=("mystic-install-state.json (Transaction state marker)")
}

backup_file() {
    local target_file="$1"
    local target_tx_dir="${TX_DIR:-}"
    
    if [ -z "$target_tx_dir" ] || [ ! -d "$target_tx_dir" ]; then
        echo "Error: Cannot back up '$target_file'. Transaction directory not initialized." >&2
        return 1
    fi
    
    target_file=$(_to_posix_path "$target_file")
    local backup_dir="${TX_BACKUP_DIR:-${target_tx_dir}/backup}"
    mkdir -p "$backup_dir" 2>/dev/null
    
    if [ ! -e "$target_file" ]; then
        _add_to_backup_manifest "$target_file" "null" "NOT_PRESENT" "$target_tx_dir"
        record_tx_step "backup_file" "NOT_PRESENT" "$target_file"
        return 0
    fi
    
    # Generate safe filename using hash to prevent path traversal
    local safe_hash
    if command -v md5sum >/dev/null 2>&1; then
        safe_hash=$(echo -n "$target_file" | md5sum | awk '{print $1}')
    elif command -v sha256sum >/dev/null 2>&1; then
        safe_hash=$(echo -n "$target_file" | sha256sum | awk '{print $1}')
    else
        safe_hash=$(echo -n "$target_file" | tr '/\\ ' '___')
    fi
    
    local backup_dest="${backup_dir}/${safe_hash}.bak"
    backup_dest=$(_to_posix_path "$backup_dest")
    
    # Perform copy preserving permissions where possible
    if cp -p "$target_file" "$backup_dest" 2>/dev/null || cp "$target_file" "$backup_dest" 2>/dev/null; then
        # Verification: copy must exist and be readable
        if [ -f "$backup_dest" ] && [ -r "$backup_dest" ]; then
            _add_to_backup_manifest "$target_file" "$backup_dest" "BACKED_UP" "$target_tx_dir"
            record_tx_step "backup_file" "BACKED_UP" "$target_file"
            return 0
        else
            echo "Error: Backup verification failed for '$target_file'." >&2
            _add_to_backup_manifest "$target_file" "$backup_dest" "VERIFICATION_FAILED" "$target_tx_dir"
            record_tx_step "backup_file" "FAILED" "Verification failed for $target_file"
            return 1
        fi
    else
        echo "Error: Failed to copy '$target_file' to '$backup_dest'." >&2
        _add_to_backup_manifest "$target_file" "$backup_dest" "COPY_FAILED" "$target_tx_dir"
        record_tx_step "backup_file" "FAILED" "Copy failed for $target_file"
        return 1
    fi
}

verify_backup() {
    local target_tx_dir="${1:-${TX_DIR:-}}"
    local manifest_file="${target_tx_dir}/backup-manifest.json"
    
    if [ ! -f "$manifest_file" ]; then
        echo "Error: Backup manifest '$manifest_file' not found." >&2
        return 1
    fi
    
    local missing_count=0
    
    if command -v jq >/dev/null 2>&1; then
        while IFS=$'\t' read -r src backup status; do
            src=$(_to_posix_path "$src")
            backup=$(_to_posix_path "$backup")
            status="${status%$'\r'}"
            if [ "$status" = "BACKED_UP" ]; then
                if [ -z "$backup" ] || [ "$backup" = "null" ] || [ ! -f "$backup" ] || [ ! -r "$backup" ]; then
                    echo "Error: Verification failed for '$src' (backup file '$backup' missing or unreadable)." >&2
                    missing_count=$((missing_count + 1))
                fi
            fi
        done < <(jq -r '.files[]? | select(.status=="BACKED_UP") | "\(.source)\t\(.backup)\t\(.status)"' "$manifest_file")
    elif command -v python3 >/dev/null 2>&1; then
        missing_count=$(python3 -c 'import json, sys, os
manifest = sys.argv[1]
missing = 0
try:
    with open(manifest, "r") as f:
        data = json.load(f)
    for item in data.get("files", []):
        if item.get("status") == "BACKED_UP":
            b = item.get("backup")
            if b:
                b = b.replace(chr(92), "/")
            if not b or not os.path.isfile(b) or not os.access(b, os.R_OK):
                sys.stderr.write("Error: Verification failed for " + str(item.get("source")) + "\n")
                missing += 1
except Exception as e:
    sys.stderr.write("Error reading manifest: " + str(e) + "\n")
    missing += 1
print(missing)
' "$manifest_file")
    fi
    
    if [ -n "$missing_count" ] && [ "$missing_count" -gt 0 ] 2>/dev/null; then
        return 1
    fi
    return 0
}

restore_backup() {
    local target_tx_dir="${1:-${TX_DIR:-}}"
    local manifest_file="${target_tx_dir}/backup-manifest.json"
    
    if [ ! -f "$manifest_file" ]; then
        echo "Error: Backup manifest '$manifest_file' not found." >&2
        return 1
    fi
    
    if ! verify_backup "$target_tx_dir"; then
        echo "Error: Backup verification failed prior to restoration." >&2
        return 1
    fi
    
    local restore_failures=0
    
    if command -v jq >/dev/null 2>&1; then
        while IFS=$'\t' read -r src backup status; do
            src=$(_to_posix_path "$src")
            backup=$(_to_posix_path "$backup")
            status="${status%$'\r'}"
            if [ "$status" = "BACKED_UP" ] && [ -n "$backup" ] && [ "$backup" != "null" ]; then
                local parent_dir
                parent_dir=$(dirname "$src")
                mkdir -p "$parent_dir" 2>/dev/null || true
                
                if cp -p "$backup" "$src" 2>/dev/null || cp "$backup" "$src" 2>/dev/null; then
                    record_tx_step "restore_file" "RESTORED" "$src"
                else
                    echo "Error: Failed to restore '$src' from '$backup'." >&2
                    record_tx_step "restore_file" "FAILED" "Failed to restore $src"
                    restore_failures=$((restore_failures + 1))
                fi
            fi
        done < <(jq -r '.files[]? | select(.status=="BACKED_UP") | "\(.source)\t\(.backup)\t\(.status)"' "$manifest_file")
    elif command -v python3 >/dev/null 2>&1; then
        while IFS=$'\t' read -r src backup status; do
            src=$(_to_posix_path "$src")
            backup=$(_to_posix_path "$backup")
            status="${status%$'\r'}"
            if [ "$status" = "BACKED_UP" ] && [ -n "$backup" ] && [ "$backup" != "null" ]; then
                local parent_dir
                parent_dir=$(dirname "$src")
                mkdir -p "$parent_dir" 2>/dev/null || true
                
                if cp -p "$backup" "$src" 2>/dev/null || cp "$backup" "$src" 2>/dev/null; then
                    record_tx_step "restore_file" "RESTORED" "$src"
                else
                    echo "Error: Failed to restore '$src' from '$backup'." >&2
                    record_tx_step "restore_file" "FAILED" "Failed to restore $src"
                    restore_failures=$((restore_failures + 1))
                fi
            fi
        done < <(python3 -c 'import json, sys
manifest = sys.argv[1]
try:
    with open(manifest, "r") as f:
        data = json.load(f)
    for item in data.get("files", []):
        if item.get("status") == "BACKED_UP":
            src = (item.get("source") or "").replace(chr(92), "/")
            bak = (item.get("backup") or "").replace(chr(92), "/")
            st = item.get("status", "")
            print(src + "\t" + bak + "\t" + st)
except Exception:
    pass
' "$manifest_file")
    fi
    
    if [ "$restore_failures" -gt 0 ]; then
        return 1
    fi
    return 0
}

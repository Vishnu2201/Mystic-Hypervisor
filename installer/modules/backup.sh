#!/usr/bin/env bash
# Backup Subsystem & State Pre-Change Engine for Mystic Hypervisor

prepare_backup_plan() {
    BACKUP_DIR="${MYSTIC_BACKUP_DIR:-/var/lib/mystic/backups}/${TX_ID:-TX_DRYRUN}"
    
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
    if [ ! -f "$target_file" ]; then
        return 0
    fi
    # Non-destructive simulation in Milestone 2
    record_tx_step "backup_file" "PREPARED" "$target_file"
}

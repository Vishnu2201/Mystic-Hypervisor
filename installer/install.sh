#!/usr/bin/env bash
# Mystic Hypervisor Installer Entrypoint
# Supports: --dry-run, --help, --version

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source modules
if [ -f "$SCRIPT_DIR/modules/detection.sh" ]; then
    source "$SCRIPT_DIR/modules/detection.sh"
    source "$SCRIPT_DIR/modules/compatibility.sh"
    source "$SCRIPT_DIR/modules/backup.sh"
fi

IS_DRY_RUN=0

for arg in "$@"; do
    case "$arg" in
        --dry-run)
            IS_DRY_RUN=1
            ;;
        --help|-h)
            echo "Mystic Hypervisor Installer"
            echo "Usage: install.sh [--dry-run]"
            echo "  --dry-run    Inspect local system and output installation plan without making any host changes."
            exit 0
            ;;
        --version|-v)
            echo "Mystic Hypervisor Installer v0.1.0-foundation"
            exit 0
            ;;
    esac
done

if [ "$IS_DRY_RUN" -eq 1 ]; then
    echo "========================================================"
    echo "       MYSTIC HYPERVISOR INSTALLER — DRY RUN MODE        "
    echo "========================================================"
    echo "[NOTICE] Dry run mode enabled. No changes will be made to host system."
    echo ""
    
    detect_system
    evaluate_compatibility
    prepare_backup_plan

    echo ""
    echo "=== Planned Modifications (DRY RUN PLAN) ==="
    echo "  1. Create directory /etc/mystic/ and /var/lib/mystic/"
    echo "  2. Install mysticd daemon binary to /usr/local/bin/mysticd"
    echo "  3. Install mysticctl CLI binary to /usr/local/bin/mysticctl"
    echo "  4. Configure Incus hypervisor backend (if selected)"
    echo "  5. Register mysticd systemd service"
    echo "  6. Generate self-signed TLS certificates"
    echo "  7. Run initial health check via mysticctl doctor"
    echo ""
    echo "Dry run completed successfully. System remains untouched."
    exit 0
fi

echo "Interactive host installation is scheduled for Milestone 2."
echo "Use 'bash installer/install.sh --dry-run' to preview system inspection and installation plan."

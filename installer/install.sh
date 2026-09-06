#!/usr/bin/env bash
# Mystic Hypervisor Safe Installer Entrypoint
# Modes: --dry-run, --plan, --apply, --rollback, --doctor, --uninstall, --version, --help

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source Modules
if [ -f "$SCRIPT_DIR/modules/detection.sh" ]; then
    source "$SCRIPT_DIR/modules/detection.sh"
    source "$SCRIPT_DIR/modules/compatibility.sh"
    source "$SCRIPT_DIR/modules/pkgmanager.sh"
    source "$SCRIPT_DIR/modules/transaction.sh"
    source "$SCRIPT_DIR/modules/backup.sh"
    source "$SCRIPT_DIR/modules/netsafety.sh"
    source "$SCRIPT_DIR/modules/plan.sh"
fi

MODE=""
CONFIRM_YES=0
OUTPUT_JSON=0

show_help() {
    echo "Mystic Hypervisor Installer"
    echo "Usage: ./install.sh [MODE] [OPTIONS]"
    echo ""
    echo "Modes:"
    echo "  --dry-run       Perform read-only system inspection and print installation plan (Default Safe Mode)."
    echo "  --plan          Generate structured installation plan (--json for machine-readable format)."
    echo "  --apply         Apply installation transaction to host server (Requires confirmation)."
    echo "  --rollback      Roll back the most recent recorded installation transaction."
    echo "  --doctor        Run diagnostic health checks on host system."
    echo "  --uninstall     Safely remove Mystic control plane (Preserves user VMs & storage)."
    echo "  --version       Display installer version information."
    echo "  --help          Display this help message."
    echo ""
    echo "Options:"
    echo "  --yes, -y       Automatically confirm --apply transaction prompt."
    echo "  --json          Output plan in JSON format (Used with --plan)."
}

# Parse Command-Line Arguments
if [ "$#" -eq 0 ]; then
    echo "========================================================"
    echo "            MYSTIC HYPERVISOR SAFE INSTALLER            "
    echo "========================================================"
    echo "Mystic Hypervisor manages real Linux virtualization infrastructure."
    echo "To protect system stability, default invocation requires an explicit mode."
    echo ""
    echo "Recommended start command (Read-Only System Inspection):"
    echo "  ./install.sh --dry-run"
    echo ""
    echo "Run './install.sh --help' for full mode options."
    exit 0
fi

while [ "$#" -gt 0 ]; do
    case "$1" in
        --dry-run)
            MODE="dry-run"
            ;;
        --plan)
            MODE="plan"
            ;;
        --apply)
            MODE="apply"
            ;;
        --rollback)
            MODE="rollback"
            ;;
        --doctor)
            MODE="doctor"
            ;;
        --uninstall)
            MODE="uninstall"
            ;;
        --version|-v)
            echo "Mystic Hypervisor Safe Installer v0.5.0-milestone3e"
            exit 0
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        --yes|-y)
            CONFIRM_YES=1
            ;;
        --json)
            OUTPUT_JSON=1
            ;;
        *)
            echo "Error: Unknown argument '$1'. Run './install.sh --help' for options." >&2
            exit 1
            ;;
    esac
    shift
done

# Perform Preflight System Inspection
detect_system
evaluate_compatibility
detect_package_manager

if [ "$MODE" = "apply" ] && [ "${IS_LINUX:-0}" -eq 0 ]; then
    echo "[ERROR] Cannot execute --apply on a non-Linux development host (${UNAME_S:-UNKNOWN})." >&2
    echo "[ERROR] Mystic Hypervisor --apply requires a supported target Linux server." >&2
    exit 1
fi

init_transaction "${MODE:-dry-run}"
prepare_backup_plan
verify_network_safety

# Execute Mode Logic
case "$MODE" in
    dry-run)
        render_dry_run_plan
        exit 0
        ;;
    plan)
        if [ "$OUTPUT_JSON" -eq 1 ]; then
            render_json_plan
        else
            render_dry_run_plan
        fi
        exit 0
        ;;
    apply)
        if [ "${IS_LINUX:-0}" -eq 0 ]; then
            echo "[ERROR] Cannot execute --apply on a non-Linux development host ($UNAME_S)." >&2
            echo "[ERROR] Mystic Hypervisor --apply requires a supported target Linux server." >&2
            exit 1
        fi

        if [ "$CONFIRM_YES" -ne 1 ]; then
            render_dry_run_plan
            echo "========================================================"
            read -r -p "PROCEED WITH INSTALLATION ON THIS HOST? [y/N] " response
            case "$response" in
                [yY][eE][sS]|[yY])
                    ;;
                *)
                    echo "Installation cancelled by user. System untouched."
                    exit 0
                    ;;
            esac
        fi

        echo "Beginning installation transaction ${TX_ID}..."
        record_tx_step "preflight_validation" "PASSED" "Linux $DETECTED_ID"
        echo "Milestone 2 transaction model recorded. Package transactions staged."
        exit 0
        ;;
    rollback)
        echo "=== Mystic Hypervisor Transaction Rollback ==="
        if [ "${IS_LINUX:-0}" -eq 0 ]; then
            echo "[NOTICE] Non-Linux dev host. No transactions to roll back."
            exit 0
        fi
        echo "Inspecting transaction history..."
        get_transaction_summary
        echo "Rollback engine staged."
        exit 0
        ;;
    doctor)
        if [ -f "$SCRIPT_DIR/doctor.sh" ]; then
            bash "$SCRIPT_DIR/doctor.sh"
        else
            echo "Doctor diagnostic module missing." >&2
            exit 1
        fi
        exit 0
        ;;
    uninstall)
        if [ -f "$SCRIPT_DIR/uninstall.sh" ]; then
            bash "$SCRIPT_DIR/uninstall.sh"
        else
            echo "Uninstall module missing." >&2
            exit 1
        fi
        exit 0
        ;;
    *)
        show_help
        exit 0
        ;;
esac

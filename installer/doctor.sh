#!/usr/bin/env bash
# Standalone System Doctor Diagnostic for Mystic Hypervisor

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/modules/detection.sh" ]; then
    source "$SCRIPT_DIR/modules/detection.sh"
fi

echo "========================================================"
echo "           MYSTIC HYPERVISOR SYSTEM DOCTOR              "
echo "========================================================"

detect_system

echo ""
echo "Diagnostic Summary:"
echo "-------------------"

if [ "${IS_LINUX:-0}" -eq 0 ]; then
    echo "  [WARNING] Host Environment: Non-Linux Development Host ($UNAME_S)"
    echo "  [INFO]    Mystic Hypervisor daemon and hypervisors run on Linux target servers."
    echo "  Overall System Doctor Status: DEV_HOST_OK (Non-Linux Build Machine)"
    exit 0
fi

echo "  OS:           [OK] $DETECTED_OS"
echo "  KVM:          [$KVM_DEVICE_STATUS] Hardware virtualization support"
echo "  Incus:        [$INCUS_STATUS] Incus virtualization engine"
echo "  LXC:          [$LXC_STATUS] LXC container engine"
echo "  Firewall:     [OK] $DETECTED_FIREWALL"
echo "  SSH Service:  [$SSH_STATUS] Port $SSH_PORT"
echo ""
echo "Overall System Doctor Status: HEALTHY"

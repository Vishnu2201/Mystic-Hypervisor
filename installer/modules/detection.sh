#!/usr/bin/env bash
# System Detection Module for Mystic Hypervisor

detect_system() {
    echo "=== Mystic Hypervisor System Inspection ==="
    
    UNAME_S="$(uname -s)"
    IS_LINUX=0

    case "$UNAME_S" in
        Linux*)
            IS_LINUX=1
            ;;
        MINGW*|MSYS*|CYGWIN*)
            echo "  [NOTICE] Detected OS: $UNAME_S (Windows Development Host)"
            echo "  [NOTICE] Mystic Hypervisor is designed exclusively for Linux servers."
            echo "  [NOTICE] Windows/Git Bash is suitable for local development/building."
            echo "  [NOTICE] Target system inspection requires execution on a supported Linux distribution."
            echo ""
            ;;
        *)
            echo "  [WARNING] Unsupported non-Linux OS detected: $UNAME_S"
            echo ""
            ;;
    esac

    if [ "$IS_LINUX" -eq 1 ] && [ -f /etc/os-release ]; then
        . /etc/os-release
        DETECTED_OS="$NAME $VERSION"
        DETECTED_ID="$ID"
    else
        DETECTED_OS="$UNAME_S (Non-Linux Dev Environment)"
        DETECTED_ID="non-linux"
    fi

    echo "  OS Distribution:          $DETECTED_OS"
    echo "  Kernel Version:           $(uname -r)"
    echo "  Architecture:             $(uname -m)"

    # Hardware Specs
    if [ "$IS_LINUX" -eq 1 ] && [ -f /proc/cpuinfo ]; then
        DETECTED_CPU_COUNT=$(grep -c ^processor /proc/cpuinfo)
    else
        DETECTED_CPU_COUNT=1
    fi
    echo "  CPU Cores:                $DETECTED_CPU_COUNT"

    if [ "$IS_LINUX" -eq 1 ] && [ -f /proc/meminfo ]; then
        DETECTED_RAM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
        DETECTED_RAM_MB=$((DETECTED_RAM_KB / 1024))
    else
        DETECTED_RAM_MB=0
    fi
    if [ "$DETECTED_RAM_MB" -gt 0 ]; then
        echo "  System Memory (RAM):      ${DETECTED_RAM_MB} MB"
    else
        echo "  System Memory (RAM):      N/A (Non-Linux Dev Environment)"
    fi

    # Disk Space Inspection
    if [ "$IS_LINUX" -eq 1 ]; then
        DETECTED_DISK_FREE=$(df -h / | awk 'NR==2 {print $4}')
        echo "  Root Disk Available:      $DETECTED_DISK_FREE"
    else
        echo "  Root Disk Available:      N/A (Non-Linux Dev Environment)"
    fi

    # Network Inspection
    if [ "$IS_LINUX" -eq 1 ]; then
        DETECTED_DEFAULT_IF=$(ip route show default 2>/dev/null | awk '/default/ {print $5}' || echo "unknown")
        echo "  Primary Network IF:       $DETECTED_DEFAULT_IF"
    else
        echo "  Primary Network IF:       N/A (Non-Linux Dev Environment)"
    fi

    # Virtualization Capability Check
    HAS_KVM="NO"
    if [ -e /dev/kvm ]; then
        HAS_KVM="YES"
    fi
    echo "  Hardware KVM Available:   $HAS_KVM"

    # Existing Hypervisors
    INCUS_PRESENT="NO"
    if command -v incus >/dev/null 2>&1; then INCUS_PRESENT="YES"; fi
    LXC_PRESENT="NO"
    if command -v lxc >/dev/null 2>&1; then LXC_PRESENT="YES"; fi

    echo "  Incus Installed:          $INCUS_PRESENT"
    echo "  LXC Installed:            $LXC_PRESENT"
}

#!/usr/bin/env bash
# System Detection Module for Mystic Hypervisor
# Detects OS, Kernel, Architecture, CPU, RAM, Disk, Network, Virt, SSH, and Firewall state.

detect_system() {
    UNAME_S="$(uname -s 2>/dev/null || echo "UNKNOWN")"
    IS_LINUX=0

    case "$UNAME_S" in
        Linux*)
            IS_LINUX=1
            ;;
        MINGW*|MSYS*|CYGWIN*)
            DETECTED_OS="$UNAME_S (Windows Development Host)"
            DETECTED_ID="windows_dev"
            ;;
        *)
            DETECTED_OS="$UNAME_S (Unsupported OS)"
            DETECTED_ID="unsupported"
            ;;
    esac

    if [ "$IS_LINUX" -eq 1 ] && [ -f /etc/os-release ]; then
        # Parse os-release safely
        DETECTED_OS_NAME=$(grep '^NAME=' /etc/os-release | cut -d= -f2 | tr -d '"')
        DETECTED_OS_VERSION=$(grep '^VERSION=' /etc/os-release | cut -d= -f2 | tr -d '"')
        DETECTED_ID=$(grep '^ID=' /etc/os-release | cut -d= -f2 | tr -d '"')
        DETECTED_OS="$DETECTED_OS_NAME $DETECTED_OS_VERSION"
    fi

    DETECTED_KERNEL="$(uname -r 2>/dev/null || echo "UNKNOWN")"
    DETECTED_ARCH="$(uname -m 2>/dev/null || echo "UNKNOWN")"

    # Boot Mode (UEFI vs BIOS)
    if [ "$IS_LINUX" -eq 1 ] && [ -d /sys/firmware/efi ]; then
        DETECTED_BOOT_MODE="UEFI"
    elif [ "$IS_LINUX" -eq 1 ]; then
        DETECTED_BOOT_MODE="BIOS/Legacy"
    else
        DETECTED_BOOT_MODE="UNAVAILABLE (Dev Host)"
    fi

    # CPU Detection
    if [ "$IS_LINUX" -eq 1 ] && [ -f /proc/cpuinfo ]; then
        DETECTED_CPU_COUNT=$(grep -c ^processor /proc/cpuinfo 2>/dev/null || echo "1")
        DETECTED_CPU_MODEL=$(grep 'model name' /proc/cpuinfo 2>/dev/null | head -n1 | cut -d: -f2 | sed 's/^[ \t]*//' || echo "Generic CPU")
    else
        DETECTED_CPU_COUNT=1
        DETECTED_CPU_MODEL="Development Host CPU"
    fi

    # Memory & Swap Detection
    if [ "$IS_LINUX" -eq 1 ] && [ -f /proc/meminfo ]; then
        RAM_KB=$(grep MemTotal /proc/meminfo 2>/dev/null | awk '{print $2}' || echo "0")
        SWAP_KB=$(grep SwapTotal /proc/meminfo 2>/dev/null | awk '{print $2}' || echo "0")
        DETECTED_RAM_MB=$((RAM_KB / 1024))
        DETECTED_SWAP_MB=$((SWAP_KB / 1024))
    else
        DETECTED_RAM_MB=0
        DETECTED_SWAP_MB=0
    fi

    # Disk Space & Filesystem Detection
    if [ "$IS_LINUX" -eq 1 ]; then
        DETECTED_DISK_FREE=$(df -h / 2>/dev/null | awk 'NR==2 {print $4}' || echo "UNKNOWN")
        DETECTED_DISK_TOTAL=$(df -h / 2>/dev/null | awk 'NR==2 {print $2}' || echo "UNKNOWN")
        DETECTED_FS_TYPE=$(stat -f -c %T / 2>/dev/null || echo "UNKNOWN")
    else
        DETECTED_DISK_FREE="UNAVAILABLE"
        DETECTED_DISK_TOTAL="UNAVAILABLE"
        DETECTED_FS_TYPE="UNAVAILABLE"
    fi

    # Virtualization Support (/dev/kvm & Nested Virt)
    HAS_KVM="NO"
    if [ -e /dev/kvm ]; then
        HAS_KVM="YES"
    fi

    NESTED_VIRT="NO"
    if [ -f /sys/module/kvm_intel/parameters/nested ]; then
        NESTED_VAL=$(cat /sys/module/kvm_intel/parameters/nested 2>/dev/null || echo "N")
        if [ "$NESTED_VAL" = "Y" ] || [ "$NESTED_VAL" = "1" ]; then NESTED_VIRT="YES"; fi
    elif [ -f /sys/module/kvm_amd/parameters/nested ]; then
        NESTED_VAL=$(cat /sys/module/kvm_amd/parameters/nested 2>/dev/null || echo "0")
        if [ "$NESTED_VAL" = "1" ]; then NESTED_VIRT="YES"; fi
    fi

    # Installed Hypervisors & Container Runtimes
    INCUS_PRESENT="NO"
    if command -v incus >/dev/null 2>&1; then INCUS_PRESENT="YES"; fi
    LXC_PRESENT="NO"
    if command -v lxc >/dev/null 2>&1; then LXC_PRESENT="YES"; fi
    LIBVIRT_PRESENT="NO"
    if command -v virsh >/dev/null 2>&1 || [ -f /etc/init.d/libvirtd ]; then LIBVIRT_PRESENT="YES"; fi
    QEMU_PRESENT="NO"
    if command -v qemu-system-x86_64 >/dev/null 2>&1; then QEMU_PRESENT="YES"; fi

    DOCKER_PRESENT="NO"
    if command -v docker >/dev/null 2>&1; then DOCKER_PRESENT="YES"; fi
    PODMAN_PRESENT="NO"
    if command -v podman >/dev/null 2>&1; then PODMAN_PRESENT="YES"; fi

    # Network Inspection
    if [ "$IS_LINUX" -eq 1 ]; then
        DETECTED_MGMT_IF=$(ip route show default 2>/dev/null | awk '/default/ {print $5}' | head -n1 || echo "UNKNOWN")
        DETECTED_DEFAULT_ROUTE=$(ip route show default 2>/dev/null | awk '/default/ {print $3}' | head -n1 || echo "UNKNOWN")
        DETECTED_PRIVATE_IP=$(ip -4 addr show dev "$DETECTED_MGMT_IF" 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | head -n1 || echo "UNKNOWN")
        DETECTED_PUBLIC_IP=$(curl -s --max-time 2 https://api.ipify.org 2>/dev/null || echo "UNCHECKED")
        DETECTED_BRIDGES=$(ip link show type bridge 2>/dev/null | awk -F': ' '{print $2}' | tr '\n' ' ' || echo "NONE")
    else
        DETECTED_MGMT_IF="UNAVAILABLE"
        DETECTED_DEFAULT_ROUTE="UNAVAILABLE"
        DETECTED_PRIVATE_IP="UNAVAILABLE"
        DETECTED_PUBLIC_IP="UNAVAILABLE"
        DETECTED_BRIDGES="NONE"
    fi

    # DNS Configuration
    if [ -f /etc/resolv.conf ]; then
        DETECTED_DNS=$(grep '^nameserver' /etc/resolv.conf 2>/dev/null | awk '{print $2}' | tr '\n' ' ' || echo "UNKNOWN")
    else
        DETECTED_DNS="UNAVAILABLE"
    fi

    # Firewall Inspection
    DETECTED_FIREWALL="NONE"
    if command -v nft >/dev/null 2>&1 && nft list ruleset >/dev/null 2>&1; then
        DETECTED_FIREWALL="nftables"
    elif command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "active"; then
        DETECTED_FIREWALL="ufw"
    elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
        DETECTED_FIREWALL="firewalld"
    elif command -v iptables >/dev/null 2>&1; then
        DETECTED_FIREWALL="iptables"
    fi

    # SSH Inspection
    SSH_STATUS="UNKNOWN"
    SSH_PORT="22"
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl is-active --quiet ssh 2>/dev/null || systemctl is-active --quiet sshd 2>/dev/null; then
            SSH_STATUS="RUNNING"
        fi
    fi
    if [ -f /etc/ssh/sshd_config ]; then
        CONFIG_PORT=$(grep -i '^Port ' /etc/ssh/sshd_config 2>/dev/null | awk '{print $2}' || echo "")
        if [ -n "$CONFIG_PORT" ]; then SSH_PORT="$CONFIG_PORT"; fi
    fi
}

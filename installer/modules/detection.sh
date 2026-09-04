#!/usr/bin/env bash
# Advanced Deep Inspection & Coexistence Detection Module for Mystic Hypervisor (Milestone 3D)
# Performs 100% read-only inspection of Host, KVM, Incus, Docker, Pterodactyl, Network, Storage, Ownership, and NAT/Public IP topology.

is_private_ipv4() {
    local ip="$1"
    case "$ip" in
        10.*|192.168.*|127.*|169.254.*) return 0 ;;
        172.1[6-9].*|172.2[0-9].*|172.3[01].*) return 0 ;;
        100.6[4-9].*|100.[7-9][0-9].*|100.1[01][0-9].*|100.12[0-7].*) return 0 ;;
        *) return 1 ;;
    esac
}

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
        DETECTED_OS_NAME=$(grep '^NAME=' /etc/os-release | cut -d= -f2 | tr -d '"')
        DETECTED_OS_VERSION=$(grep '^VERSION=' /etc/os-release | cut -d= -f2 | tr -d '"')
        DETECTED_ID=$(grep '^ID=' /etc/os-release | cut -d= -f2 | tr -d '"')
        DETECTED_OS="$DETECTED_OS_NAME $DETECTED_OS_VERSION"
    fi

    DETECTED_KERNEL="$(uname -r 2>/dev/null || echo "UNKNOWN")"
    DETECTED_ARCH="$(uname -m 2>/dev/null || echo "UNKNOWN")"
    DETECTED_HOSTNAME="$(hostname -f 2>/dev/null || hostname 2>/dev/null || echo "localhost")"

    # Machine ID (Non-Secret Hash Status)
    if [ -f /etc/machine-id ]; then
        MACHINE_ID_HASH=$(sha256sum /etc/machine-id 2>/dev/null | awk '{print $1}' | cut -c1-8 || echo "PRESENT")
        MACHINE_ID_STATUS="PRESENT (Hash: ${MACHINE_ID_HASH}...)"
    else
        MACHINE_ID_STATUS="NOT_PRESENT"
    fi

    # Boot Mode & Uptime & Systemd & cgroup
    if [ "$IS_LINUX" -eq 1 ] && [ -d /sys/firmware/efi ]; then
        DETECTED_BOOT_MODE="UEFI"
    elif [ "$IS_LINUX" -eq 1 ]; then
        DETECTED_BOOT_MODE="BIOS/Legacy"
    else
        DETECTED_BOOT_MODE="UNAVAILABLE (Dev Host)"
    fi

    if [ "$IS_LINUX" -eq 1 ]; then
        UPTIME_STRING="$(uptime -p 2>/dev/null || uptime 2>/dev/null || echo "UNKNOWN")"
        SYSTEMD_STATUS="$(command -v systemctl >/dev/null 2>&1 && echo "AVAILABLE" || echo "NOT_AVAILABLE")"
        if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
            CGROUP_VERSION="v2"
        else
            CGROUP_VERSION="v1"
        fi
        VIRT_ENVIRONMENT="$(systemd-detect-virt 2>/dev/null || echo "none/baremetal")"
    else
        UPTIME_STRING="UNAVAILABLE"
        SYSTEMD_STATUS="UNAVAILABLE"
        CGROUP_VERSION="UNAVAILABLE"
        VIRT_ENVIRONMENT="windows_dev_host"
    fi

    # CPU Detection & Topology & Virt Flags
    if [ "$IS_LINUX" -eq 1 ] && [ -f /proc/cpuinfo ]; then
        DETECTED_CPU_COUNT=$(grep -c ^processor /proc/cpuinfo 2>/dev/null || echo "1")
        DETECTED_CPU_MODEL=$(grep 'model name' /proc/cpuinfo 2>/dev/null | head -n1 | cut -d: -f2 | sed 's/^[ \t]*//' || echo "Generic CPU")
        CPU_SOCKETS=$(grep 'physical id' /proc/cpuinfo 2>/dev/null | sort -u | wc -l 2>/dev/null || echo "1")
        if [ "$CPU_SOCKETS" -eq 0 ]; then CPU_SOCKETS=1; fi
        CPU_TOPOLOGY="Sockets: ${CPU_SOCKETS}, Cores: ${DETECTED_CPU_COUNT}"

        if grep -qE 'vmx|svm' /proc/cpuinfo 2>/dev/null; then
            CPU_VIRT_FLAGS="PRESENT"
        else
            CPU_VIRT_FLAGS="NOT_PRESENT"
        fi
    else
        DETECTED_CPU_COUNT=1
        DETECTED_CPU_MODEL="Development Host CPU"
        CPU_TOPOLOGY="Sockets: 1, Cores: 1"
        CPU_VIRT_FLAGS="UNKNOWN"
    fi

    # Memory & Swap & Load Average
    if [ "$IS_LINUX" -eq 1 ] && [ -f /proc/meminfo ]; then
        RAM_KB=$(grep MemTotal /proc/meminfo 2>/dev/null | awk '{print $2}' || echo "0")
        SWAP_KB=$(grep SwapTotal /proc/meminfo 2>/dev/null | awk '{print $2}' || echo "0")
        DETECTED_RAM_MB=$((RAM_KB / 1024))
        DETECTED_SWAP_MB=$((SWAP_KB / 1024))
        LOAD_AVG="$(cat /proc/loadavg 2>/dev/null | awk '{print $1 ", " $2 ", " $3}' || echo "UNKNOWN")"
    else
        DETECTED_RAM_MB=0
        DETECTED_SWAP_MB=0
        LOAD_AVG="UNAVAILABLE"
    fi

    # Multi-Source Filesystem Detection
    if [ "$IS_LINUX" -eq 1 ]; then
        DETECTED_DISK_FREE=$(df -h / 2>/dev/null | awk 'NR==2 {print $4}' || echo "UNKNOWN")
        DETECTED_DISK_TOTAL=$(df -h / 2>/dev/null | awk 'NR==2 {print $2}' || echo "UNKNOWN")
        DETECTED_INODE_USAGE=$(df -i / 2>/dev/null | awk 'NR==2 {print $5 " (" $3 "/" $2 ")"}' || echo "UNKNOWN")

        FS_FINDMNT=$(findmnt -n -o FSTYPE / 2>/dev/null || echo "")
        FS_DF=$(df -T / 2>/dev/null | awk 'NR==2 {print $2}' || echo "")
        FS_STAT=$(stat -f -c %T / 2>/dev/null || echo "")

        if [ -n "$FS_FINDMNT" ]; then
            DETECTED_FS_TYPE="$FS_FINDMNT"
        elif [ -n "$FS_DF" ]; then
            DETECTED_FS_TYPE="$FS_DF"
        else
            DETECTED_FS_TYPE="${FS_STAT:-UNKNOWN}"
        fi

        BLOCK_DEVICES="$(lsblk -d -n -o NAME,SIZE,TYPE 2>/dev/null | tr '\n' ';' || echo "UNKNOWN")"
    else
        DETECTED_DISK_FREE="UNAVAILABLE"
        DETECTED_DISK_TOTAL="UNAVAILABLE"
        DETECTED_INODE_USAGE="UNAVAILABLE"
        DETECTED_FS_TYPE="UNAVAILABLE"
        BLOCK_DEVICES="UNAVAILABLE"
    fi

    # KVM Capability & Detailed Reasons
    if [ "$IS_LINUX" -eq 1 ]; then
        if [ -r /dev/kvm ] && [ -w /dev/kvm ]; then
            KVM_DEVICE_STATUS="ACCESSIBLE"
            KVM_UNAVAILABLE_REASON="None. KVM acceleration is fully accessible."
        elif [ -e /dev/kvm ]; then
            KVM_DEVICE_STATUS="PRESENT_INACCESSIBLE"
            KVM_UNAVAILABLE_REASON="Permissions error accessing /dev/kvm (Check udev/group membership)."
        else
            KVM_DEVICE_STATUS="NOT_PRESENT"
            KVM_UNAVAILABLE_REASON="/dev/kvm device node missing. Typical on Cloud VPS instances where upstream nested virtualization is disabled."
        fi
    else
        KVM_DEVICE_STATUS="UNAVAILABLE (Dev Host)"
        KVM_UNAVAILABLE_REASON="Non-Linux development machine."
    fi

    NESTED_VIRT="UNKNOWN"
    if [ "$IS_LINUX" -eq 1 ]; then
        if [ -f /sys/module/kvm_intel/parameters/nested ]; then
            NESTED_VAL=$(cat /sys/module/kvm_intel/parameters/nested 2>/dev/null || echo "N")
            if [ "$NESTED_VAL" = "Y" ] || [ "$NESTED_VAL" = "1" ]; then NESTED_VIRT="AVAILABLE"; else NESTED_VIRT="NOT_AVAILABLE"; fi
        elif [ -f /sys/module/kvm_amd/parameters/nested ]; then
            NESTED_VAL=$(cat /sys/module/kvm_amd/parameters/nested 2>/dev/null || echo "0")
            if [ "$NESTED_VAL" = "1" ]; then NESTED_VIRT="AVAILABLE"; else NESTED_VIRT="NOT_AVAILABLE"; fi
        fi
    fi

    # Virtualization Stack Binary Statuses
    INCUS_STATUS="NOT_INSTALLED"
    if command -v incus >/dev/null 2>&1; then INCUS_STATUS="INSTALLED"; fi

    LXC_STATUS="NOT_INSTALLED"
    if command -v lxc >/dev/null 2>&1; then LXC_STATUS="INSTALLED"; fi

    QEMU_STATUS="NOT_INSTALLED"
    if command -v qemu-system-x86_64 >/dev/null 2>&1; then QEMU_STATUS="INSTALLED"; fi

    LIBVIRT_STATUS="NOT_INSTALLED"
    if command -v virsh >/dev/null 2>&1 || [ -f /etc/init.d/libvirtd ]; then LIBVIRT_STATUS="INSTALLED"; fi

    DOCKER_STATUS="NOT_INSTALLED"
    if command -v docker >/dev/null 2>&1; then DOCKER_STATUS="INSTALLED"; fi

    PODMAN_STATUS="NOT_INSTALLED"
    if command -v podman >/dev/null 2>&1; then PODMAN_STATUS="INSTALLED"; fi

    # Incus Read-Only Deep Inspection
    INCUS_VERSION="NONE"
    INCUS_STORAGE_POOLS="NONE"
    INCUS_NETWORKS="NONE"
    INCUS_PROJECTS="NONE"
    INCUS_PROFILES="NONE"
    INCUS_INSTANCES_COUNT=0

    if [ "$INCUS_STATUS" = "INSTALLED" ] && [ "$IS_LINUX" -eq 1 ]; then
        INCUS_VERSION=$(incus version 2>/dev/null | tail -n1 || echo "UNKNOWN")
        INCUS_STORAGE_POOLS=$(incus storage list --format json 2>/dev/null | grep -o '"name":"[^"]*"' | cut -d'"' -f4 | tr '\n' ' ' || echo "DEFAULT/UNKNOWN")
        INCUS_NETWORKS=$(incus network list --format json 2>/dev/null | grep -o '"name":"[^"]*"' | cut -d'"' -f4 | tr '\n' ' ' || echo "DEFAULT/UNKNOWN")
        INCUS_PROJECTS=$(incus project list --format json 2>/dev/null | grep -o '"name":"[^"]*"' | cut -d'"' -f4 | tr '\n' ' ' || echo "DEFAULT")
        INCUS_PROFILES=$(incus profile list --format json 2>/dev/null | grep -o '"name":"[^"]*"' | cut -d'"' -f4 | tr '\n' ' ' || echo "DEFAULT")
        INCUS_INSTANCES_COUNT=$(incus list --format json 2>/dev/null | grep -c '"name"' || echo "0")
    fi

    # Docker Read-Only Deep Inspection
    DOCKER_VERSION="NONE"
    DOCKER_RUNNING_COUNT=0
    DOCKER_TOTAL_COUNT=0
    DOCKER_NETWORKS="NONE"
    DOCKER_VOLUMES_COUNT=0

    if [ "$DOCKER_STATUS" = "INSTALLED" ] && [ "$IS_LINUX" -eq 1 ]; then
        DOCKER_VERSION=$(docker --version 2>/dev/null | head -n1 || echo "UNKNOWN")
        DOCKER_RUNNING_COUNT=$(docker ps -q 2>/dev/null | wc -l || echo "0")
        DOCKER_TOTAL_COUNT=$(docker ps -a -q 2>/dev/null | wc -l || echo "0")
        DOCKER_NETWORKS=$(docker network ls --format '{{.Name}}' 2>/dev/null | tr '\n' ' ' || echo "bridge host none")
        DOCKER_VOLUMES_COUNT=$(docker volume ls -q 2>/dev/null | wc -l || echo "0")
    fi

    # Pterodactyl Service Detection
    PTERODACTYL_DETECTED="NO"
    if [ "$IS_LINUX" -eq 1 ]; then
        if systemctl is-active --quiet wings 2>/dev/null || [ -d /etc/pterodactyl ] || [ -d /srv/daemon-data ]; then
            PTERODACTYL_DETECTED="YES"
        fi
    fi

    # Network Inventory & Unambiguous Public/Private IP Ownership Detection
    if [ "$IS_LINUX" -eq 1 ]; then
        DETECTED_MGMT_IF=$(ip route show default 2>/dev/null | awk '/default/ {print $5}' | head -n1 || echo "UNKNOWN")
        DETECTED_DEFAULT_ROUTE=$(ip route show default 2>/dev/null | awk '/default/ {print $3}' | head -n1 || echo "UNKNOWN")

        # Inspect all IPv4 addresses assigned directly to local interfaces
        LOCAL_IPS=$(ip -4 addr show 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '^127\.' || echo "")

        DETECTED_PRIVATE_IP="UNKNOWN"
        DETECTED_HOST_PUBLIC_IP="NOT_ASSIGNED"
        PUBLIC_IP_ASSIGNMENT_STATUS="NOT_ASSIGNED"

        for ip_addr in $LOCAL_IPS; do
            if is_private_ipv4 "$ip_addr"; then
                if [ "$DETECTED_PRIVATE_IP" = "UNKNOWN" ]; then
                    DETECTED_PRIVATE_IP="$ip_addr"
                fi
            else
                # Found a globally routable public IPv4 address assigned to a local interface!
                DETECTED_HOST_PUBLIC_IP="$ip_addr"
                PUBLIC_IP_ASSIGNMENT_STATUS="DIRECT"
            fi
        done

        # Optional read-only external IP lookup (Evidence of UPSTREAM public gateway IP only!)
        DETECTED_UPSTREAM_PUBLIC_IP=$(curl -s --max-time 2 https://api.ipify.org 2>/dev/null || echo "UNAVAILABLE")

        if [ "$PUBLIC_IP_ASSIGNMENT_STATUS" = "DIRECT" ]; then
            NAT_STATUS="NOT_DETECTED"
            DETECTED_NETWORK_TOPOLOGY="DIRECT_PUBLIC"
        elif [ "$DETECTED_PRIVATE_IP" != "UNKNOWN" ]; then
            NAT_STATUS="LIKELY"
            if [ "$DETECTED_UPSTREAM_PUBLIC_IP" != "UNAVAILABLE" ] && [ "$DETECTED_UPSTREAM_PUBLIC_IP" != "UNKNOWN" ]; then
                DETECTED_NETWORK_TOPOLOGY="NAT_LIKELY"
            else
                DETECTED_NETWORK_TOPOLOGY="PRIVATE_ONLY"
            fi
        else
            NAT_STATUS="UNKNOWN"
            DETECTED_NETWORK_TOPOLOGY="UNKNOWN"
        fi

        CONFIGURED_EXPOSURE_MODE="UNCONFIGURED"
        CONFIGURED_GATEWAY_ID=""
        CONFIGURED_GATEWAY_PUBLIC_IP=""

        # Read-only Bridge Inventory & Ownership Mapping
        RAW_BRIDGES=$(ip link show type bridge 2>/dev/null | awk -F': ' '{print $2}' | awk '{print $1}' || echo "")

        BRIDGE_DETAILS=()
        if [ -n "$RAW_BRIDGES" ]; then
            for br in $RAW_BRIDGES; do
                local owner="EXTERNAL"
                case "$br" in
                    docker0|docker*) owner="DOCKER" ;;
                    pterodactyl*|ptero*) owner="PTERODACTYL"; PTERODACTYL_DETECTED="YES" ;;
                    incusbr*|incus*) owner="INCUS" ;;
                    virbr*) owner="LIBVIRT" ;;
                    mysticbr*) owner="MYSTIC" ;;
                    *) owner="EXTERNAL" ;;
                esac
                BRIDGE_DETAILS+=("$br ($owner)")
            done
        fi
        DETECTED_BRIDGES_STRING="${BRIDGE_DETAILS[*]:-NONE}"
    else
        DETECTED_MGMT_IF="UNAVAILABLE"
        DETECTED_DEFAULT_ROUTE="UNAVAILABLE"
        DETECTED_PRIVATE_IP="UNAVAILABLE"
        DETECTED_HOST_PUBLIC_IP="UNAVAILABLE"
        DETECTED_UPSTREAM_PUBLIC_IP="UNAVAILABLE"
        PUBLIC_IP_ASSIGNMENT_STATUS="UNKNOWN"
        NAT_STATUS="UNKNOWN"
        DETECTED_NETWORK_TOPOLOGY="UNKNOWN"
        CONFIGURED_EXPOSURE_MODE="UNCONFIGURED"
        CONFIGURED_GATEWAY_ID=""
        CONFIGURED_GATEWAY_PUBLIC_IP=""
        DETECTED_BRIDGES_STRING="NONE"
    fi

    # Backwards compatibility alias: DETECTED_PUBLIC_IP matches DETECTED_HOST_PUBLIC_IP
    DETECTED_PUBLIC_IP="$DETECTED_HOST_PUBLIC_IP"

    # DNS Configuration
    if [ -f /etc/resolv.conf ]; then
        DETECTED_DNS=$(grep '^nameserver' /etc/resolv.conf 2>/dev/null | awk '{print $2}' | tr '\n' ' ' || echo "UNKNOWN")
    else
        DETECTED_DNS="UNAVAILABLE"
    fi

    # Listening Ports (Read-Only ss scan)
    LISTEN_PORTS=()
    if [ "$IS_LINUX" -eq 1 ] && command -v ss >/dev/null 2>&1; then
        LISTEN_PORTS+=($(ss -tulpn 2>/dev/null | awk 'NR>1 {print $5}' | awk -F: '{print $NF}' | sort -u -n | head -n 12 || echo ""))
    fi
    DETECTED_LISTEN_PORTS="${LISTEN_PORTS[*]:-UNKNOWN}"

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

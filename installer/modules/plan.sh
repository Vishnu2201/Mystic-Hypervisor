#!/usr/bin/env bash
# Plan Generation & Output Renderer Module for Mystic Hypervisor

render_dry_run_plan() {
    echo "========================================================"
    echo "              MYSTIC HYPERVISOR INSTALLER              "
    echo "========================================================"
    echo ""

    if [ "${IS_LINUX:-0}" -eq 0 ]; then
        echo "[NOTICE] Environment: Non-Linux Development Host ($UNAME_S)"
        echo "[NOTICE] Mystic Hypervisor is a Linux server product."
        echo "[NOTICE] Target system inspection requires execution on a supported Linux server host."
        echo ""
    fi

    echo "Environment"
    echo "-----------"
    echo "OS:            ${DETECTED_OS}"
    echo "Distribution:  ${DETECTED_ID}"
    echo "Kernel:        ${DETECTED_KERNEL}"
    echo "Architecture:  ${DETECTED_ARCH}"
    echo "Boot Mode:     ${DETECTED_BOOT_MODE}"
    echo ""

    echo "Resources"
    echo "---------"
    echo "CPU Cores:     ${DETECTED_CPU_COUNT} (${DETECTED_CPU_MODEL})"
    if [ "${DETECTED_RAM_MB:-0}" -gt 0 ]; then
        echo "RAM:           ${DETECTED_RAM_MB} MB"
        echo "Swap:          ${DETECTED_SWAP_MB} MB"
        echo "Disk Free:     ${DETECTED_DISK_FREE} / ${DETECTED_DISK_TOTAL} (${DETECTED_FS_TYPE})"
    else
        echo "RAM:           N/A (Dev Host)"
        echo "Swap:          N/A (Dev Host)"
        echo "Disk:          N/A (Dev Host)"
    fi
    echo ""

    echo "Virtualization"
    echo "--------------"
    echo "KVM:           ${HAS_KVM} (Nested: ${NESTED_VIRT})"
    echo "Incus:         ${INCUS_PRESENT}"
    echo "LXC:           ${LXC_PRESENT}"
    echo "QEMU:          ${QEMU_PRESENT}"
    echo "libvirt:       ${LIBVIRT_PRESENT}"
    echo "Docker:        ${DOCKER_PRESENT}"
    echo "Podman:        ${PODMAN_PRESENT}"
    echo ""

    echo "Networking"
    echo "----------"
    echo "Management IF: ${DETECTED_MGMT_IF}"
    echo "Private IP:    ${DETECTED_PRIVATE_IP}"
    echo "Public IP:     ${DETECTED_PUBLIC_IP}"
    echo "Default route: ${DETECTED_DEFAULT_ROUTE}"
    echo "Bridges:       ${DETECTED_BRIDGES}"
    echo "Firewall:      ${DETECTED_FIREWALL}"
    echo "SSH Service:   ${SSH_STATUS} (Port: ${SSH_PORT})"
    echo ""

    echo "Compatibility"
    echo "-------------"
    echo "Profile:       ${RESOURCE_PROFILE}"
    echo "Recommended:   ${RECOMMENDED_PROVIDERS}"
    echo "Rating:        ${SYSTEM_RATING}"
    if [ "${#WARNINGS[@]}" -gt 0 ]; then
        echo "Warnings:"
        for w in "${WARNINGS[@]}"; do
            echo "  - $w"
        done
    else
        echo "Warnings:      None"
    fi
    echo ""

    echo "Installation Plan"
    echo "-----------------"
    echo "[ ] System directories: /etc/mystic/, /var/lib/mystic/"
    echo "[ ] Mystic Control Plane binary: /usr/local/bin/mysticd"
    echo "[ ] Mystic CLI binary: /usr/local/bin/mysticctl"
    echo "[ ] Virtualization Provider Driver: ${RECOMMENDED_PROVIDERS}"
    echo "[ ] Service Registration: mysticd.service (systemd)"
    echo "[ ] TLS Certificates: Self-signed / Custom certificate setup"
    echo ""

    echo "Potential Risks"
    echo "---------------"
    if [ "${#NET_SAFETY_RISKS[@]}" -gt 0 ]; then
        for r in "${NET_SAFETY_RISKS[@]}"; do
            echo "  - $r"
        done
    else
        echo "  - None identified for initial safe installation."
    fi
    echo ""

    echo "NO CHANGES WILL BE MADE."
    echo ""
    echo "Exit code:"
    echo "0 = compatible/read-only success"
    echo "non-zero = detection/validation error"
}

render_json_plan() {
    cat <<EOF
{
  "installer_version": "0.2.0-milestone2",
  "environment": {
    "os": "$DETECTED_OS",
    "distribution_id": "$DETECTED_ID",
    "kernel": "$DETECTED_KERNEL",
    "architecture": "$DETECTED_ARCH",
    "is_linux": $IS_LINUX
  },
  "resources": {
    "cpu_cores": $DETECTED_CPU_COUNT,
    "ram_mb": $DETECTED_RAM_MB,
    "swap_mb": $DETECTED_SWAP_MB
  },
  "virtualization": {
    "kvm": "$HAS_KVM",
    "incus": "$INCUS_PRESENT",
    "lxc": "$LXC_PRESENT"
  },
  "compatibility": {
    "profile": "$RESOURCE_PROFILE",
    "recommended_providers": "$RECOMMENDED_PROVIDERS",
    "rating": "$SYSTEM_RATING"
  },
  "dry_run": true
}
EOF
}

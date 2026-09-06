#!/usr/bin/env bash
# Plan Generation & Coexistence Output Renderer Module for Mystic Hypervisor
#
# ARCHITECTURAL LIFECYCLE BOUNDARIES:
# - detection: System discovery, OS identification & capability inspection
# - planning: Installation spec creation, network exposure validation & dry-run planning (THIS MODULE)
# - transaction state: Persistent transaction logging, audit history & state machine
# - backup: Pre-change file backups, verification & restoration
# - future mutation: (Milestone 4B/4C) Host system package installation, binary placement & systemd service setup

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

    echo "Host Overview"
    echo "-------------"
    echo "Hostname:      ${DETECTED_HOSTNAME}"
    echo "Machine ID:    ${MACHINE_ID_STATUS}"
    echo "OS:            ${DETECTED_OS}"
    echo "Distribution:  ${DETECTED_ID}"
    echo "Kernel:        ${DETECTED_KERNEL}"
    echo "Architecture:  ${DETECTED_ARCH}"
    echo "Boot Mode:     ${DETECTED_BOOT_MODE}"
    echo "Uptime:        ${UPTIME_STRING}"
    echo "Systemd:       ${SYSTEMD_STATUS}"
    echo "cgroup:        ${CGROUP_VERSION}"
    echo "Virt Env:      ${VIRT_ENVIRONMENT}"
    echo ""

    echo "Hardware Resources"
    echo "------------------"
    echo "CPU Model:     ${DETECTED_CPU_MODEL}"
    echo "CPU Topology:  ${CPU_TOPOLOGY}"
    echo "CPU Virt Flags:${CPU_VIRT_FLAGS}"
    if [ "${DETECTED_RAM_MB:-0}" -gt 0 ]; then
        echo "RAM:           ${DETECTED_RAM_MB} MB"
        echo "Swap:          ${DETECTED_SWAP_MB} MB"
        echo "Load Average:  ${LOAD_AVG}"
        echo "Root Storage:  ${DETECTED_DISK_FREE} free / ${DETECTED_DISK_TOTAL} total (${DETECTED_FS_TYPE})"
        echo "Inodes Used:   ${DETECTED_INODE_USAGE}"
        echo "Block Devices: ${BLOCK_DEVICES}"
    else
        echo "RAM:           N/A (Dev Host)"
        echo "Swap:          N/A (Dev Host)"
        echo "Load Average:  N/A (Dev Host)"
        echo "Root Storage:  N/A (Dev Host)"
    fi
    echo ""

    echo "KVM & Hardware Virtualization"
    echo "-----------------------------"
    echo "KVM Device:    ${KVM_DEVICE_STATUS}"
    echo "KVM Reason:    ${KVM_UNAVAILABLE_REASON}"
    echo "CPU Flags:     ${CPU_VIRT_FLAGS}"
    echo "Nested Virt:   ${NESTED_VIRT}"
    echo ""

    echo "Virtualization & Coexisting Infrastructure"
    echo "----------------------------------------"
    echo "Incus Status:  ${INCUS_STATUS} (Version: ${INCUS_VERSION})"
    if [ "$INCUS_STATUS" = "INSTALLED" ]; then
        echo "  - Storage Pools: ${INCUS_STORAGE_POOLS}"
        echo "  - Networks:      ${INCUS_NETWORKS}"
        echo "  - Projects:      ${INCUS_PROJECTS}"
        echo "  - Profiles:      ${INCUS_PROFILES}"
        echo "  - Instances:     ${INCUS_INSTANCES_COUNT}"
    fi
    echo "Docker Status: ${DOCKER_STATUS} (Version: ${DOCKER_VERSION})"
    if [ "$DOCKER_STATUS" = "INSTALLED" ]; then
        echo "  - Containers:    ${DOCKER_RUNNING_COUNT} running / ${DOCKER_TOTAL_COUNT} total"
        echo "  - Networks:      ${DOCKER_NETWORKS}"
        echo "  - Volumes:       ${DOCKER_VOLUMES_COUNT}"
    fi
    echo "Pterodactyl:   ${PTERODACTYL_DETECTED}"
    echo "QEMU Status:   ${QEMU_STATUS}"
    echo "LXC Status:    ${LXC_STATUS}"
    echo "libvirt:       ${LIBVIRT_STATUS}"
    echo "Podman:        ${PODMAN_STATUS}"
    echo ""

    echo "Networking & IP Topology"
    echo "------------------------"
    echo "Management IF: ${DETECTED_MGMT_IF}"
    echo "Private IP:    ${DETECTED_PRIVATE_IP}"
    echo "Host Public IP:${DETECTED_HOST_PUBLIC_IP}"
    echo "Upstream IP:   ${DETECTED_UPSTREAM_PUBLIC_IP}"
    echo "IP Assignment: ${PUBLIC_IP_ASSIGNMENT_STATUS}"
    echo "NAT Status:    ${NAT_STATUS}"
    echo "Topology:      ${DETECTED_NETWORK_TOPOLOGY}"
    echo "Default Route: ${DETECTED_DEFAULT_ROUTE}"
    echo "Bridges:       ${DETECTED_BRIDGES_STRING}"
    echo "DNS:           ${DETECTED_DNS}"
    echo "Firewall:      ${DETECTED_FIREWALL}"
    echo "SSH Status:    ${SSH_STATUS} (Port: ${SSH_PORT})"
    echo "Listen Ports:  ${DETECTED_LISTEN_PORTS}"
    echo ""

    echo "Network Exposure Model"
    echo "----------------------"
    echo "Supported Modes:"
    echo "  - PRIVATE_ONLY      (Host reachable via private network / VPN only)"
    echo "  - NAT_FORWARDED     (Private host behind upstream gateway / port forwarding)"
    echo "  - DIRECT_PUBLIC     (Host with direct public IP on interface)"
    echo "  - EXTERNAL_GATEWAY  (Traffic routed through dedicated external gateway / proxy)"
    echo ""
    echo "Current Detected State: ${DETECTED_NETWORK_TOPOLOGY} (NAT Status: ${NAT_STATUS})"
    echo "Configured Exposure Mode: ${CONFIGURED_EXPOSURE_MODE}"
    echo "Gateway ID:               ${CONFIGURED_GATEWAY_ID:-NONE}"
    echo "Gateway Public IP:        ${CONFIGURED_GATEWAY_PUBLIC_IP:-NONE}"
    echo "Note: Observed network facts are distinct from configured exposure modes."
    echo "No network, routing, firewall, or port forwarding changes will be made."
    echo ""

    echo "Mystic Safety & Resource Ownership"
    echo "----------------------------------"
    echo "Management Interface:           PROTECTED (${DETECTED_MGMT_IF})"
    echo "Existing Network Bridges:       PRESERVED (${DETECTED_BRIDGES_STRING})"
    echo "Existing Incus Configuration:   PRESERVED"
    echo "Existing Workloads:             PRESERVED"
    echo "Ownership Model:                Mystic owns MYSTIC resources only. External infrastructure defaults to EXPLICIT_SUBSYSTEM / UNKNOWN."
    echo ""

    echo "Compatibility & Provider Recommendation"
    echo "---------------------------------------"
    echo "Profile:       ${RESOURCE_PROFILE}"
    echo "Rating:        ${SYSTEM_RATING}"
    echo "Recommended:   ${RECOMMENDED_PROVIDERS}"
    if [ "${#RECOMMENDATION_REASONS[@]}" -gt 0 ]; then
        echo "Reason:"
        for r in "${RECOMMENDATION_REASONS[@]}"; do
            echo "  - $r"
        done
    fi
    if [ "${#WARNINGS[@]}" -gt 0 ]; then
        echo "Warnings:"
        for w in "${WARNINGS[@]}"; do
            echo "  - $w"
        done
    fi
    echo ""

    echo "Installation Plan"
    echo "-----------------"
    echo "[ ] System directories: /etc/mystic/, /var/lib/mystic/"
    echo "[ ] Mystic Control Plane binary: /usr/local/bin/mysticd"
    echo "[ ] Mystic CLI binary: /usr/local/bin/mysticctl"
    echo "[ ] Virtualization Driver Binding: ${RECOMMENDED_PROVIDERS}"
    echo "[ ] Service Registration: mysticd.service (systemd)"
    echo "[ ] TLS Certificates: Self-signed / Custom certificate setup"
    echo ""

    echo "Potential Risks & Safety Guards"
    echo "--------------------------------"
    if [ "${#NET_SAFETY_RISKS[@]}" -gt 0 ]; then
        for r in "${NET_SAFETY_RISKS[@]}"; do
            echo "  - $r"
        done
    else
        echo "  - None identified. Host infrastructure will be preserved."
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
  "installer_version": "0.5.0-milestone3e",
  "host": {
    "hostname": "$DETECTED_HOSTNAME",
    "machine_id_status": "$MACHINE_ID_STATUS",
    "os": "$DETECTED_OS",
    "distribution_id": "$DETECTED_ID",
    "kernel": "$DETECTED_KERNEL",
    "architecture": "$DETECTED_ARCH",
    "boot_mode": "$DETECTED_BOOT_MODE",
    "uptime": "$UPTIME_STRING",
    "systemd_status": "$SYSTEMD_STATUS",
    "cgroup_version": "$CGROUP_VERSION",
    "virt_environment": "$VIRT_ENVIRONMENT",
    "is_linux": $IS_LINUX
  },
  "resources": {
    "cpu_cores": $DETECTED_CPU_COUNT,
    "cpu_model": "$DETECTED_CPU_MODEL",
    "cpu_topology": "$CPU_TOPOLOGY",
    "cpu_virt_flags": "$CPU_VIRT_FLAGS",
    "ram_mb": $DETECTED_RAM_MB,
    "swap_mb": $DETECTED_SWAP_MB,
    "load_avg": "$LOAD_AVG",
    "disk_free": "$DETECTED_DISK_FREE",
    "disk_total": "$DETECTED_DISK_TOTAL",
    "fs_type": "$DETECTED_FS_TYPE",
    "inode_usage": "$DETECTED_INODE_USAGE",
    "block_devices": "$BLOCK_DEVICES"
  },
  "kvm_capability": {
    "kvm_device_status": "$KVM_DEVICE_STATUS",
    "kvm_unavailable_reason": "$KVM_UNAVAILABLE_REASON",
    "cpu_virt_flags": "$CPU_VIRT_FLAGS",
    "nested_virt": "$NESTED_VIRT"
  },
  "virtualization_stack": {
    "incus": {
      "status": "$INCUS_STATUS",
      "version": "$INCUS_VERSION",
      "storage_pools": "$INCUS_STORAGE_POOLS",
      "networks": "$INCUS_NETWORKS",
      "projects": "$INCUS_PROJECTS",
      "profiles": "$INCUS_PROFILES",
      "instances_count": $INCUS_INSTANCES_COUNT
    },
    "docker": {
      "status": "$DOCKER_STATUS",
      "version": "$DOCKER_VERSION",
      "running_containers": $DOCKER_RUNNING_COUNT,
      "total_containers": $DOCKER_TOTAL_COUNT,
      "networks": "$DOCKER_NETWORKS",
      "volumes_count": $DOCKER_VOLUMES_COUNT
    },
    "pterodactyl_detected": "$PTERODACTYL_DETECTED",
    "qemu_status": "$QEMU_STATUS",
    "lxc_status": "$LXC_STATUS",
    "libvirt_status": "$LIBVIRT_STATUS",
    "podman_status": "$PODMAN_STATUS"
  },
  "networking": {
    "detected": {
      "management_interface": "$DETECTED_MGMT_IF",
      "private_ip": "$DETECTED_PRIVATE_IP",
      "host_public_ip": "$DETECTED_HOST_PUBLIC_IP",
      "upstream_public_ip": "$DETECTED_UPSTREAM_PUBLIC_IP",
      "public_ip_assignment_status": "$PUBLIC_IP_ASSIGNMENT_STATUS",
      "nat_status": "$NAT_STATUS",
      "topology": "$DETECTED_NETWORK_TOPOLOGY",
      "default_route": "$DETECTED_DEFAULT_ROUTE",
      "bridges": "$DETECTED_BRIDGES_STRING",
      "dns": "$DETECTED_DNS",
      "firewall": "$DETECTED_FIREWALL",
      "ssh_status": "$SSH_STATUS",
      "ssh_port": "$SSH_PORT",
      "listen_ports": "$DETECTED_LISTEN_PORTS"
    },
    "configuration": {
      "exposure_mode": "$CONFIGURED_EXPOSURE_MODE",
      "gateway_id": "$CONFIGURED_GATEWAY_ID",
      "gateway_public_ip": "$CONFIGURED_GATEWAY_PUBLIC_IP",
      "forwarding_rules": []
    },
    "management_interface": "$DETECTED_MGMT_IF",
    "private_ip": "$DETECTED_PRIVATE_IP",
    "host_public_ip": "$DETECTED_HOST_PUBLIC_IP",
    "upstream_public_ip": "$DETECTED_UPSTREAM_PUBLIC_IP",
    "public_ip_assignment_status": "$PUBLIC_IP_ASSIGNMENT_STATUS",
    "nat_status": "$NAT_STATUS",
    "detected_network_topology": "$DETECTED_NETWORK_TOPOLOGY",
    "configured_exposure_mode": "$CONFIGURED_EXPOSURE_MODE",
    "default_route": "$DETECTED_DEFAULT_ROUTE",
    "bridges": "$DETECTED_BRIDGES_STRING",
    "dns": "$DETECTED_DNS",
    "firewall": "$DETECTED_FIREWALL",
    "ssh_status": "$SSH_STATUS",
    "ssh_port": "$SSH_PORT",
    "listen_ports": "$DETECTED_LISTEN_PORTS"
  },
  "resource_ownership": {
    "management_interface": "SYSTEM / PROTECTED",
    "existing_bridges": "PRESERVED",
    "existing_incus_config": "PRESERVED",
    "existing_docker_workloads": "PRESERVED",
    "default_ownership": "UNKNOWN / EXTERNAL"
  },
  "compatibility": {
    "profile": "$RESOURCE_PROFILE",
    "rating": "$SYSTEM_RATING",
    "recommended_providers": "$RECOMMENDED_PROVIDERS"
  },
  "dry_run": true
}
EOF
}

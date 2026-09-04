#!/usr/bin/env bash
# Resource Classification & System Compatibility Module for Mystic Hypervisor

evaluate_compatibility() {
    WARNINGS=()

    if [ "${IS_LINUX:-0}" -eq 0 ]; then
        RESOURCE_PROFILE="Development Host (Non-Linux)"
        RECOMMENDED_PROVIDERS="Target Linux Server Required"
        SYSTEM_RATING="UNSUPPORTED_DEV_HOST"
        WARNINGS+=("Installer executed on non-Linux development environment ($UNAME_S). Target Linux host is required.")
        return 0
    fi

    # OS Compatibility Check
    case "$DETECTED_ID" in
        ubuntu|debian|rhel|rocky|almalinux)
            OS_SUPPORTED=1
            ;;
        *)
            OS_SUPPORTED=0
            WARNINGS+=("Distribution '$DETECTED_ID' is not officially verified. Mystic officially supports Ubuntu, Debian, RHEL, Rocky Linux, AlmaLinux.")
            ;;
    esac

    # Classify resource tier based on Constitution Section 8
    if [ "$DETECTED_RAM_MB" -lt 2048 ]; then
        RESOURCE_PROFILE="Tiny Profile (< 2 GB RAM)"
        RECOMMENDED_PROVIDERS="LXC Container Only"
        SYSTEM_RATING="LIMITED"
        WARNINGS+=("System memory is under 2 GB. KVM virtual machines are NOT recommended. Container-only (LXC) workload advised.")
    elif [ "$DETECTED_RAM_MB" -lt 4096 ]; then
        RESOURCE_PROFILE="Small Profile (2 - 4 GB RAM)"
        RECOMMENDED_PROVIDERS="Incus"
        SYSTEM_RATING="SUITABLE"
    elif [ "$DETECTED_RAM_MB" -lt 8192 ]; then
        RESOURCE_PROFILE="Standard Profile (4 - 8 GB RAM)"
        RECOMMENDED_PROVIDERS="Incus + KVM"
        SYSTEM_RATING="RECOMMENDED"
    else
        RESOURCE_PROFILE="Large Profile (8+ GB RAM)"
        RECOMMENDED_PROVIDERS="Incus + KVM"
        SYSTEM_RATING="EXCELLENT"
    fi

    if [ "$HAS_KVM" != "YES" ]; then
        WARNINGS+=("Hardware KVM acceleration (/dev/kvm) is unavailable. Full VM performance will be degraded (qemu emulation required).")
    fi

    if [ "$DETECTED_CPU_COUNT" -lt 2 ]; then
        WARNINGS+=("System has only 1 CPU core. Multi-workload hypervisor performance will be constrained.")
    fi
}

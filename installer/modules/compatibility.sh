#!/usr/bin/env bash
# Resource Classification & Coexistence Recommendation Module for Mystic Hypervisor (Milestone 3C)

evaluate_compatibility() {
    WARNINGS=()
    RECOMMENDATION_REASONS=()

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
        SYSTEM_RATING="LIMITED"
        WARNINGS+=("System memory is under 2 GB. KVM virtual machines are NOT recommended. Container-only workload advised.")
    elif [ "$DETECTED_RAM_MB" -lt 4096 ]; then
        RESOURCE_PROFILE="Small Profile (2 - 4 GB RAM)"
        SYSTEM_RATING="SUITABLE"
    elif [ "$DETECTED_RAM_MB" -lt 8192 ]; then
        RESOURCE_PROFILE="Standard Profile (4 - 8 GB RAM)"
        SYSTEM_RATING="RECOMMENDED"
    else
        RESOURCE_PROFILE="Large Profile (8+ GB RAM)"
        SYSTEM_RATING="EXCELLENT"
    fi

    # Coexistence & Multi-Factor Provider Recommendation Engine
    if [ "$INCUS_STATUS" = "INSTALLED" ] && [ "$KVM_DEVICE_STATUS" != "ACCESSIBLE" ]; then
        RECOMMENDED_PROVIDERS="Incus"
        RECOMMENDATION_REASONS+=("Incus is already installed on the host server.")
        RECOMMENDATION_REASONS+=("Pre-existing Incus network configuration detected ($INCUS_NETWORKS).")
        RECOMMENDATION_REASONS+=("KVM hardware acceleration (/dev/kvm) is unavailable ($KVM_UNAVAILABLE_REASON).")
        if [ "$QEMU_STATUS" = "INSTALLED" ]; then
            RECOMMENDATION_REASONS+=("QEMU binary being installed does not supply hardware KVM acceleration without /dev/kvm.")
        fi
        RECOMMENDATION_REASONS+=("Host resources ($DETECTED_CPU_COUNT Cores, ${DETECTED_RAM_MB} MB RAM) are more than sufficient for Incus workloads.")
        if [ "$DOCKER_STATUS" = "INSTALLED" ] || [ "$PTERODACTYL_DETECTED" = "YES" ]; then
            RECOMMENDATION_REASONS+=("Existing Docker and Pterodactyl infrastructure will remain untouched.")
        fi
        RECOMMENDATION_REASONS+=("Mystic will integrate with existing Incus rather than attempting replacement.")
    elif [ "$KVM_DEVICE_STATUS" = "ACCESSIBLE" ] && [ "$DETECTED_RAM_MB" -ge 4096 ]; then
        RECOMMENDED_PROVIDERS="Incus + KVM"
        RECOMMENDATION_REASONS+=("KVM hardware acceleration (/dev/kvm) is accessible.")
        RECOMMENDATION_REASONS+=("Host memory ($DETECTED_RAM_MB MB) supports mixed VM and Container workloads.")
    elif [ "$INCUS_STATUS" = "INSTALLED" ]; then
        RECOMMENDED_PROVIDERS="Incus"
        RECOMMENDATION_REASONS+=("Incus hypervisor is already present on host.")
    elif [ "$DETECTED_RAM_MB" -lt 2048 ]; then
        RECOMMENDED_PROVIDERS="LXC Container Only"
        RECOMMENDATION_REASONS+=("Resource constrained profile: LXC containers provide minimal CPU/RAM overhead.")
    else
        RECOMMENDED_PROVIDERS="Incus"
        RECOMMENDATION_REASONS+=("Incus provides general-purpose system container and VM management.")
    fi

    if [ "$KVM_DEVICE_STATUS" != "ACCESSIBLE" ]; then
        WARNINGS+=("KVM acceleration unavailable: $KVM_UNAVAILABLE_REASON")
    fi

    if [ "$DETECTED_CPU_COUNT" -lt 2 ]; then
        WARNINGS+=("System has only 1 CPU core. Multi-workload hypervisor performance will be constrained.")
    fi
}

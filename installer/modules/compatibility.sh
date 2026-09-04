#!/usr/bin/env bash
# Compatibility & Resource Classification Module for Mystic Hypervisor

evaluate_compatibility() {
    echo ""
    echo "=== Compatibility & Recommendation ==="
    
    if [ "${IS_LINUX:-0}" -eq 0 ]; then
        RESOURCE_PROFILE="Development Environment (Non-Linux)"
        RECOMMENDED_PROVIDER="Target Linux Host Required"
        RATING="UNSUPPORTED_DEV_HOST"
        echo "  Resource Tier Profile:    $RESOURCE_PROFILE"
        echo "  Recommended Provider:     $RECOMMENDED_PROVIDER"
        echo "  System Suitability:       $RATING (Installer runs on Linux servers only)"
        return 0
    fi

    # Classify resource tier based on Constitution Section 8
    if [ "$DETECTED_RAM_MB" -lt 2048 ]; then
        RESOURCE_PROFILE="Tiny (1-2 GB RAM)"
        RECOMMENDED_PROVIDER="LXC Container Only"
        RATING="LIMITED"
    elif [ "$DETECTED_RAM_MB" -lt 4096 ]; then
        RESOURCE_PROFILE="Small (2-4 GB RAM)"
        RECOMMENDED_PROVIDER="Incus"
        RATING="SUITABLE"
    elif [ "$DETECTED_RAM_MB" -lt 8192 ]; then
        RESOURCE_PROFILE="Standard (4-8 GB RAM)"
        RECOMMENDED_PROVIDER="Incus + KVM"
        RATING="RECOMMENDED"
    else
        RESOURCE_PROFILE="Large (8+ GB RAM)"
        RECOMMENDED_PROVIDER="Incus + KVM"
        RATING="EXCELLENT"
    fi

    echo "  Resource Tier Profile:    $RESOURCE_PROFILE"
    echo "  Recommended Provider:     $RECOMMENDED_PROVIDER"
    echo "  System Suitability:       $RATING"
}

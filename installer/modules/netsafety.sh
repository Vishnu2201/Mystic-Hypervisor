#!/usr/bin/env bash
# Network Safety & Coexistence Engine for Mystic Hypervisor

verify_network_safety() {
    NET_SAFETY_RISKS=()
    PRESERVED_BRIDGES=()

    if [ "${IS_LINUX:-0}" -eq 0 ]; then
        return 0
    fi

    # Identify Management Interface Protection
    if [ "$DETECTED_MGMT_IF" = "UNKNOWN" ] || [ -z "$DETECTED_MGMT_IF" ]; then
        NET_SAFETY_RISKS+=("CRITICAL: Unable to identify default management network interface. Network modifications are locked.")
    else
        NET_SAFETY_RISKS+=("SAFETY GUARD: Primary management interface '$DETECTED_MGMT_IF' (IP: $DETECTED_PRIVATE_IP, Gateway: $DETECTED_DEFAULT_ROUTE, SSH Port: $SSH_PORT) is PROTECTED.")
    fi

    # Coexistence & Bridge Preservation Guard
    if [ -n "${DETECTED_BRIDGES_STRING:-}" ] && [ "$DETECTED_BRIDGES_STRING" != "NONE" ]; then
        NET_SAFETY_RISKS+=("COEXISTENCE GUARD: Pre-existing network bridges ($DETECTED_BRIDGES_STRING) are PRESERVED as external infrastructure.")
    fi
}

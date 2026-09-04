#!/usr/bin/env bash
# Safe Package Manager Abstraction Module for Mystic Hypervisor
# Supports apt (Debian/Ubuntu) and dnf/yum (RHEL/Rocky/AlmaLinux)

detect_package_manager() {
    if command -v apt-get >/dev/null 2>&1; then
        PKG_FAMILY="apt"
        PKG_CHECK_CMD="dpkg-query -W -f='\${Status}'"
    elif command -v dnf >/dev/null 2>&1; then
        PKG_FAMILY="dnf"
        PKG_CHECK_CMD="rpm -q"
    elif command -v yum >/dev/null 2>&1; then
        PKG_FAMILY="yum"
        PKG_CHECK_CMD="rpm -q"
    else
        PKG_FAMILY="UNKNOWN"
        PKG_CHECK_CMD="false"
    fi
}

pkg_is_installed() {
    local pkg="$1"
    case "$PKG_FAMILY" in
        apt)
            dpkg-query -W -f='${Status}' "$pkg" 2>/dev/null | grep -q "ok installed"
            ;;
        dnf|yum)
            rpm -q "$pkg" >/dev/null 2>&1
            ;;
        *)
            return 1
            ;;
    esac
}

pkg_plan_install() {
    local pkgs=("$@")
    MISSING_PKGS=()
    INSTALLED_PKGS=()

    for pkg in "${pkgs[@]}"; do
        if pkg_is_installed "$pkg"; then
            INSTALLED_PKGS+=("$pkg")
        else
            MISSING_PKGS+=("$pkg")
        fi
    done
}

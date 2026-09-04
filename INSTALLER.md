# Mystic Hypervisor — Safe Installer Architecture

**Status:** Milestone 1 — Engineering Foundation  
**Reference Document:** `PROJECT_CONSTITUTION.md`

## Overview

The Mystic installer pipeline is designed around safety, predictability, and complete reversibility.

## Installer Order

```text
System Detection
      ↓
Compatibility Checks
      ↓
Existing-State Inspection
      ↓
Installation Plan (--dry-run output)
      ↓
User Confirmation
      ↓
Backup / State Snapshot
      ↓
Dependency & Service Installation
      ↓
Health Checks (`mysticctl doctor`)
```

## Modes & Execution

- `bash installer/install.sh --dry-run`: Non-destructive system inspection. Evaluates CPU, RAM, OS, KVM extensions, and existing hypervisors. Prints a detailed execution plan without modifying host packages or network settings.
- `bash installer/install.sh`: Interactive installation transaction (Milestone 2+).
- `bash installer/rollback.sh`: Undoes Mystic installation changes from backup state.
- `bash installer/uninstall.sh`: Removes Mystic daemon, UI, and CLI while preserving user VMs, containers, and storage.
- `bash installer/doctor.sh`: Standalone health diagnostic tool.

package workloads

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/audit"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
)

// ResultClassification defines the explicit outcome of a provider operation.
type ResultClassification string

const (
	ResultSuccess ResultClassification = "SUCCESS"
	ResultFailed  ResultClassification = "FAILED"
	ResultUnknown ResultClassification = "UNKNOWN"
)

// OwnershipStatus defines Mystic ownership verification status.
type OwnershipStatus string

const (
	OwnershipMystic   OwnershipStatus = "MYSTIC_OWNED"
	OwnershipExternal OwnershipStatus = "EXTERNAL"
	OwnershipUnknown  OwnershipStatus = "UNKNOWN"
)

var (
	ErrWorkloadNotFound           = errors.New("workload not found")
	ErrPlanNotApproved            = errors.New("provisioning plan not approved")
	ErrPlanInvalidated            = errors.New("provisioning plan invalidated by workload specification modification")
	ErrOwnershipConflict          = errors.New("ownership conflict: resource is external or not owned by Mystic")
	ErrWorkloadConfigConflict     = errors.New("configuration conflict: existing resource specifications differ from workload desired state")
	ErrIllegalStateTransition     = errors.New("illegal workload lifecycle state transition")
	ErrProviderCapabilityMissing  = errors.New("requested operation not supported by provider capabilities")
	ErrOperationAlreadyInProgress = errors.New("idempotency conflict: identical operation is already in progress or completed")
	ErrAlreadyManaged             = errors.New("instance is already managed by Mystic")
	ErrIncusInstanceNotFound      = errors.New("external instance not found on provider")
)

// OperationRecord tracks deterministic operation identity for idempotency.
type OperationRecord struct {
	OpKey      string               `json:"op_key"`
	WorkloadID string               `json:"workload_id"`
	OpType     string               `json:"op_type"`
	PlanHash   string               `json:"plan_hash"`
	Result     ResultClassification `json:"result"`
	ExecutedAt time.Time            `json:"executed_at"`
}

// ExecutionGuard enforces safety, idempotency, capability checks, ownership validation, timeouts, and result classification.
type ExecutionGuard struct {
	mu       sync.Mutex
	records  map[string]*OperationRecord
	timeout  time.Duration
	auditLog *audit.AuditLogger
}

// NewExecutionGuard creates an ExecutionGuard instance.
func NewExecutionGuard(timeout time.Duration) *ExecutionGuard {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &ExecutionGuard{
		records:  make(map[string]*OperationRecord),
		timeout:  timeout,
		auditLog: audit.GetLogger(),
	}
}

// ComputePlanHash computes a deterministic SHA256 hash of a workload spec.
func ComputePlanHash(w *Workload) string {
	if w == nil {
		return ""
	}
	raw := fmt.Sprintf("%s:%s:%s:%s:%d:%d:%d:%s:%s",
		w.Name, w.Provider, string(w.Type), w.Image,
		w.CPU, w.MemoryMB, w.StorageGB,
		w.HostID, w.NetworkConfig.NetworkName,
	)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:8])
}

// ComputeOperationKey calculates a deterministic idempotency key.
func (eg *ExecutionGuard) ComputeOperationKey(workloadID, opType, planHash string) string {
	if planHash == "" {
		planHash = "noplan"
	}
	return fmt.Sprintf("op:%s:%s:%s", workloadID, opType, planHash)
}

// VerifyCapability checks if the provider supports the required operation capability.
func (eg *ExecutionGuard) VerifyCapability(p interfaces.Provider, requiredCap interfaces.Capability) error {
	if p == nil {
		return interfaces.ErrProviderUnavailable
	}
	if !p.Capabilities().Has(requiredCap) {
		return fmt.Errorf("%w: provider '%s' lacks capability '%s'", ErrProviderCapabilityMissing, p.Name(), requiredCap)
	}
	return nil
}

// CheckOwnership inspects an instance to verify if it is owned by Mystic.
func CheckOwnership(inst *interfaces.Instance, expectedWorkloadID string) OwnershipStatus {
	if inst == nil {
		return OwnershipUnknown
	}
	if inst.Labels == nil {
		return OwnershipExternal
	}

	ownedVal := inst.Labels["user.mystic.owned"]
	if ownedVal == "" {
		ownedVal = inst.Labels["mystic.owned"]
	}
	wlIDVal := inst.Labels["user.mystic.workload_id"]
	if wlIDVal == "" {
		wlIDVal = inst.Labels["mystic.workload_id"]
	}

	if ownedVal == "true" && wlIDVal == expectedWorkloadID {
		return OwnershipMystic
	}
	if ownedVal == "true" || wlIDVal != "" {
		return OwnershipExternal
	}

	return OwnershipExternal
}

// RecordOperation stores an operation record for idempotency tracking.
func (eg *ExecutionGuard) RecordOperation(opKey, workloadID, opType, planHash string, res ResultClassification) {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	eg.records[opKey] = &OperationRecord{
		OpKey:      opKey,
		WorkloadID: workloadID,
		OpType:     opType,
		PlanHash:   planHash,
		Result:     res,
		ExecutedAt: time.Now(),
	}
}

// GetOperationRecord checks if an operation record already exists.
func (eg *ExecutionGuard) GetOperationRecord(opKey string) (*OperationRecord, bool) {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	rec, exists := eg.records[opKey]
	if !exists {
		return nil, false
	}
	// Return copy
	cp := *rec
	return &cp, true
}

// LogAudit emits a structured audit record for mutating operations.
func (eg *ExecutionGuard) LogAudit(workloadID, opType, providerName, resourceID, planHash, requestedState string, res ResultClassification, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	eg.auditLog.LogEvent(audit.AuditLog{
		Actor:    "system",
		Action:   fmt.Sprintf("workload.%s", strings.ToLower(opType)),
		Target:   fmt.Sprintf("workload/%s", workloadID),
		Result:   string(res),
		ClientIP: "local",
		ErrorMsg: errStr,
	})
}

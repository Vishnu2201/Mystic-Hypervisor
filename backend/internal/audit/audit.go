package audit

import (
	"sync"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
)

type AuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	ClientIP  string    `json:"client_ip"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Result    string    `json:"result"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

type AuditLogger struct {
	mu   sync.Mutex
	logs []AuditLog
}

var globalLogger = &AuditLogger{
	logs: make([]AuditLog, 0),
}

func GetLogger() *AuditLogger {
	return globalLogger
}

func (al *AuditLogger) LogEvent(event AuditLog) {
	al.mu.Lock()
	defer al.mu.Unlock()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	al.logs = append(al.logs, event)
	logging.GetLogger().Info("AUDIT_EVENT",
		"action", event.Action,
		"target", event.Target,
		"actor", event.Actor,
		"result", event.Result,
	)
}

func (al *AuditLogger) ListLogs() []AuditLog {
	al.mu.Lock()
	defer al.mu.Unlock()
	copied := make([]AuditLog, len(al.logs))
	copy(copied, al.logs)
	return copied
}

package audit

import "time"

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

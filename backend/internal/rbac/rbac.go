package rbac

// Role constants
const (
	RoleAdmin    = "administrator"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// Permission constants matching Constitution section 20
const (
	PermHostRead      = "host.read"
	PermHostManage    = "host.manage"
	PermInstRead      = "instance.read"
	PermInstCreate    = "instance.create"
	PermInstStart     = "instance.start"
	PermInstStop      = "instance.stop"
	PermInstDelete    = "instance.delete"
	PermInstConsole   = "instance.console"
	PermNetRead       = "network.read"
	PermNetManage     = "network.manage"
	PermStorageRead   = "storage.read"
	PermStorageManage = "storage.manage"
	PermUserRead      = "user.read"
	PermUserManage    = "user.manage"
	PermAuditRead     = "audit.read"
)

// PolicyEngine checks if a given role has a required permission.
type PolicyEngine struct{}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{}
}

func (pe *PolicyEngine) HasPermission(role, permission string) bool {
	if role == RoleAdmin {
		return true // Administrator has all permissions
	}

	switch role {
	case RoleOperator:
		switch permission {
		case PermHostRead, PermInstRead, PermInstCreate, PermInstStart,
			PermInstStop, PermInstConsole, PermNetRead, PermStorageRead:
			return true
		}
	case RoleViewer:
		switch permission {
		case PermHostRead, PermInstRead, PermNetRead, PermStorageRead:
			return true
		}
	}

	return false
}

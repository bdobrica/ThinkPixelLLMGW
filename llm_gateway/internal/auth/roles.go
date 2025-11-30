package auth

// Role represents an admin role for role-based access control
type Role string

const (
	// RoleAdmin has full access to all admin endpoints
	RoleAdmin Role = "admin"

	// RoleViewer has read-only access to admin endpoints
	RoleViewer Role = "viewer"
)

// String returns the string representation of the role
func (r Role) String() string {
	return string(r)
}

// IsValid checks if the role is a valid role
func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleViewer:
		return true
	default:
		return false
	}
}

// HasPermission checks if a role has permission for a required role
// Admin has all permissions, viewer only has viewer permissions
func (r Role) HasPermission(required Role) bool {
	if r == RoleAdmin {
		return true // Admin has all permissions
	}
	return r == required
}

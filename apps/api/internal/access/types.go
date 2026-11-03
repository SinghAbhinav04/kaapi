package access

import (
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
)

// PermissionInfo is one entry of the catalogue, as the console shows it.
type PermissionInfo struct {
	Key         auth.Permission
	Group       string
	Label       string
	Description string
}

// Role is a named set of permissions for one team.
type Role struct {
	ID          uuid.UUID
	Key         string
	Name        string
	Description string
	Team        domain.Role // whose lane it works, who gets alerts, the signature title
	System      bool        // Administrator and Guest: fixed
	Permissions []auth.Permission
	Members     int
}

// RoleInput creates or edits a role.
type RoleInput struct {
	Name        string
	Description string
	Team        domain.Role
	Permissions []auth.Permission
}

// Member is a staff account as the console lists it.
type Member struct {
	ID        uuid.UUID
	Email     string
	Name      string
	Phone     *string
	RoleID    uuid.UUID
	RoleName  string
	Team      domain.Role
	Active    bool
	CreatedAt time.Time
}

// NewMember is a staff account the administrator creates.
type NewMember struct {
	Email    string
	Name     string
	Phone    string
	RoleID   uuid.UUID
	Password string
}

// MemberChange edits an account; nil leaves a field as it is.
type MemberChange struct {
	Name     *string
	Phone    *string
	RoleID   *uuid.UUID
	Active   *bool
	Password *string
}

const (
	adminRoleKey = "ADMIN"
	guestRoleKey = "GUEST"

	// cacheTTL bounds how long another process keeps using old permissions after the
	// administrator changes them; the process that made the change forgets at once.
	cacheTTL = 30 * time.Second

	nameMin, nameMax = 2, 60
	descriptionMax   = 200
	minPasswordLen   = 12
)

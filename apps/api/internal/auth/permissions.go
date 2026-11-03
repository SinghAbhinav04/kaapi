package auth

import "slices"

// Permission is one thing a person may do. Routes declare the permission they need; which
// roles grant which permissions lives in the database (roles, role_permissions), edited by
// the administrator in the console. The constants here are the closed set the code checks,
// and a test keeps them in step with the permissions table.
type Permission string

const (
	PermOwnBookingsRead Permission = "own_bookings:read"
	PermBookingsRead    Permission = "bookings:read"
	PermBookingsExport  Permission = "bookings:export"
	PermRatesRead       Permission = "rates:read"
	PermRatesWrite      Permission = "rates:write"
	PermInventoryWrite  Permission = "inventory:write"
	PermPromosWrite     Permission = "promos:write"
	PermEnquiriesRead   Permission = "enquiries:read"
	PermEnquiriesAll    Permission = "enquiries:read_all_lanes"
	PermEnquiryReroute  Permission = "enquiries:reroute"
	PermEnquirySend     Permission = "enquiries:send"
	PermOutboxRead      Permission = "outbox:read"
	PermFactsWrite      Permission = "facts:write"
	PermDashboardRead   Permission = "dashboard:read"
	PermRetentionRun    Permission = "retention:run"
	PermAnalyticsRead   Permission = "analytics:read"
	PermIntegrations    Permission = "integrations:manage"
	PermUsersManage     Permission = "users:manage"
)

// staffPermissions is every permission a staff role can be granted.
var staffPermissions = []Permission{
	PermEnquiriesRead, PermEnquiriesAll, PermEnquiryReroute, PermEnquirySend,
	PermBookingsRead, PermBookingsExport, PermRatesRead, PermRatesWrite, PermInventoryWrite, PermPromosWrite,
	PermOutboxRead, PermFactsWrite, PermDashboardRead, PermAnalyticsRead, PermIntegrations, PermRetentionRun, PermUsersManage,
}

// AllPermissions is what the Administrator role holds: every staff permission.
func AllPermissions() []Permission { return slices.Clone(staffPermissions) }

// IsStaffPermission reports whether p can be granted to a staff role.
func IsStaffPermission(p Permission) bool { return slices.Contains(staffPermissions, p) }

// KnownPermissions is every permission the code checks, guest ones included.
func KnownPermissions() []Permission { return append(AllPermissions(), PermOwnBookingsRead) }

package auth

import (
	"slices"

	"kaapi/api/internal/domain"
)

// Permission is one thing a role may do. Routes declare the permission they need;
// roles are granted permissions here and nowhere else.
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
)

var staffBase = []Permission{PermBookingsRead, PermRatesRead, PermEnquiriesRead, PermEnquiryReroute, PermEnquirySend, PermOutboxRead, PermDashboardRead}

var grants = map[domain.Role][]Permission{
	domain.RoleGuest:        {PermOwnBookingsRead},
	domain.RoleReservations: append(slices.Clone(staffBase), PermRatesWrite, PermInventoryWrite, PermBookingsExport),
	domain.RoleSales:        slices.Clone(staffBase),
	domain.RoleGM: append(slices.Clone(staffBase), PermRatesWrite, PermInventoryWrite, PermPromosWrite, PermEnquiriesAll,
		PermBookingsExport, PermFactsWrite),
	domain.RoleAdmin: append(slices.Clone(staffBase), PermRatesWrite, PermInventoryWrite, PermPromosWrite, PermEnquiriesAll,
		PermBookingsExport, PermFactsWrite),
}

// Can reports whether a role holds a permission.
func Can(role domain.Role, p Permission) bool { return slices.Contains(grants[role], p) }

// PermissionsOf lists a role's permissions, for the client to hide what it can't use.
func PermissionsOf(role domain.Role) []Permission { return slices.Clone(grants[role]) }

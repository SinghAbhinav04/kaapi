package access

import "kaapi/api/internal/platform/apperr"

var (
	ErrRoleNotFound   = apperr.New(apperr.CodeNotFound, "We couldn't find that role.")
	ErrMemberNotFound = apperr.New(apperr.CodeNotFound, "We couldn't find that person.")
	ErrSystemRole     = apperr.New(apperr.CodeConflict, "The Administrator and Guest roles can't be changed.")
	ErrRoleInUse      = apperr.New(apperr.CodeConflict, "People still have this role. Give them another role first.")
	ErrRoleExists     = apperr.New(apperr.CodeConflict, "A role with that name already exists.")
	ErrEmailTaken     = apperr.New(apperr.CodeConflict, "Someone already uses that email address.")
	ErrLastAdmin      = apperr.New(apperr.CodeConflict, "At least one active Administrator must remain.")
	ErrSelfLockout    = apperr.New(apperr.CodeConflict, "You can't deactivate yourself or remove your own Administrator role.")
	ErrAdminOnly      = apperr.New(apperr.CodeForbidden, "Only an Administrator can give the Administrator role or change an Administrator's account.")
	ErrGrantBeyondOwn = apperr.New(apperr.CodeForbidden, "You can only give permissions you hold yourself.")
	ErrGuestRole      = apperr.New(apperr.CodeValidation, "Guest accounts are created by signing in on the website, not here.")
)

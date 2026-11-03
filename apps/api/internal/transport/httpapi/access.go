package httpapi

import (
	"net/http"

	"github.com/google/uuid"

	"kaapi/api/internal/access"
	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/httpx"
)

// accessOverview: GET /v1/staff/access → the permission catalogue and every role.
func (a *API) accessOverview(w http.ResponseWriter, r *http.Request) error {
	cat, err := a.access.Catalogue(r.Context())
	if err != nil {
		return err
	}
	roles, err := a.access.Roles(r.Context())
	if err != nil {
		return err
	}
	out := AccessResponse{Permissions: make([]PermissionDTO, 0, len(cat)), Roles: make([]RoleDTO, len(roles))}
	for _, p := range cat {
		if auth.IsStaffPermission(p.Key) { // guest-only permissions aren't grantable here
			out.Permissions = append(out.Permissions, PermissionDTO{Key: p.Key, Group: p.Group, Label: p.Label, Description: p.Description})
		}
	}
	for i, x := range roles {
		out.Roles[i] = roleDTO(x)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (a *API) createRole(w http.ResponseWriter, r *http.Request) error {
	var body RoleRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	role, err := a.access.CreateRole(r.Context(), principal(r.Context()).UserID, body.input())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, RoleResponse{Role: roleDTO(role)})
	return nil
}

func (a *API) updateRole(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, access.ErrRoleNotFound)
	if err != nil {
		return err
	}
	var body RoleRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	role, err := a.access.UpdateRole(r.Context(), principal(r.Context()).UserID, id, body.input())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, RoleResponse{Role: roleDTO(role)})
	return nil
}

func (a *API) deleteRole(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, access.ErrRoleNotFound)
	if err != nil {
		return err
	}
	if err := a.access.DeleteRole(r.Context(), principal(r.Context()).UserID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *API) listMembers(w http.ResponseWriter, r *http.Request) error {
	list, err := a.access.Members(r.Context())
	if err != nil {
		return err
	}
	out := MembersResponse{Members: make([]MemberDTO, len(list))}
	for i, m := range list {
		out.Members[i] = memberDTO(m)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (a *API) createMember(w http.ResponseWriter, r *http.Request) error {
	var body CreateMemberRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	m, err := a.access.CreateMember(r.Context(), principal(r.Context()).UserID,
		access.NewMember{Email: body.Email, Name: body.Name, Phone: body.Phone, RoleID: body.RoleID, Password: body.Password})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, MemberResponse{Member: memberDTO(m)})
	return nil
}

func (a *API) updateMember(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, access.ErrMemberNotFound)
	if err != nil {
		return err
	}
	var body UpdateMemberRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	m, err := a.access.UpdateMember(r.Context(), principal(r.Context()).UserID, id,
		access.MemberChange{Name: body.Name, Phone: body.Phone, RoleID: body.RoleID, Active: body.Active, Password: body.Password})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, MemberResponse{Member: memberDTO(m)})
	return nil
}

// pathID parses {id}; a malformed id is simply not found.
func pathID(r *http.Request, notFound error) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, notFound
	}
	return id, nil
}

func (b RoleRequest) input() access.RoleInput {
	return access.RoleInput{Name: b.Name, Description: b.Description, Team: domain.Role(b.Team), Permissions: b.Permissions}
}

func roleDTO(x access.Role) RoleDTO {
	perms := x.Permissions
	if x.Key == "ADMIN" {
		perms = auth.AllPermissions()
	}
	if perms == nil {
		perms = []auth.Permission{}
	}
	return RoleDTO{ID: x.ID, Key: x.Key, Name: x.Name, Description: x.Description, Team: x.Team, System: x.System,
		Permissions: perms, Members: x.Members}
}

func memberDTO(m access.Member) MemberDTO {
	return MemberDTO{ID: m.ID, Email: m.Email, Name: m.Name, Phone: m.Phone, RoleID: m.RoleID, RoleName: m.RoleName,
		Team: m.Team, Active: m.Active, CreatedAt: m.CreatedAt}
}

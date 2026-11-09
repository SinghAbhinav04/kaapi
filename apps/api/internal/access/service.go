// Package access is role-based access control kept in the database. The administrator
// creates roles, picks the permissions each grants, and assigns them to staff accounts;
// every request checks the caller's permissions through here.
package access

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/db"
)

type Service struct {
	pool  *pgxpool.Pool
	store store
	now   func() time.Time

	mu    sync.Mutex
	cache map[uuid.UUID]cached
}

type cached struct {
	perms   []auth.Permission
	expires time.Time
}

func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	return &Service{pool: pool, now: now, cache: map[uuid.UUID]cached{}}
}

// Permissions is what a person may do right now. The Administrator role has every
// permission, including ones added later; an inactive account has none.
func (s *Service) Permissions(ctx context.Context, userID uuid.UUID) ([]auth.Permission, error) {
	s.mu.Lock()
	c, ok := s.cache[userID]
	s.mu.Unlock()
	if ok && s.now().Before(c.expires) {
		return c.perms, nil
	}
	key, perms, err := s.store.grants(ctx, s.pool, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		perms, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	if key == adminRoleKey {
		perms = auth.AllPermissions()
	}
	s.mu.Lock()
	s.cache[userID] = cached{perms: perms, expires: s.now().Add(cacheTTL)}
	s.mu.Unlock()
	return perms, nil
}

// Can reports whether a person holds a permission.
func (s *Service) Can(ctx context.Context, userID uuid.UUID, p auth.Permission) (bool, error) {
	perms, err := s.Permissions(ctx, userID)
	return slices.Contains(perms, p), err
}

func (s *Service) forget() {
	s.mu.Lock()
	clear(s.cache)
	s.mu.Unlock()
}

// ---- roles

func (s *Service) Catalogue(ctx context.Context) ([]PermissionInfo, error) {
	return s.store.catalogue(ctx, s.pool)
}

func (s *Service) Roles(ctx context.Context) ([]Role, error) { return s.store.roles(ctx, s.pool) }

func (s *Service) CreateRole(ctx context.Context, actor uuid.UUID, in RoleInput) (Role, error) {
	in, key, err := validateRole(in)
	if err != nil {
		return Role{}, err
	}
	var id uuid.UUID
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.mayGrant(ctx, tx, actor, in.Permissions); err != nil {
			return err
		}
		if id, err = s.store.insertRole(ctx, tx, key, in); err != nil {
			return err
		}
		if err := s.store.setPermissions(ctx, tx, id, in.Permissions); err != nil {
			return err
		}
		return s.store.audit(ctx, tx, actor, "access.role_created", "role", id.String(), auditRole(in))
	})
	if err != nil {
		return Role{}, err
	}
	return s.store.role(ctx, s.pool, id)
}

func (s *Service) UpdateRole(ctx context.Context, actor, id uuid.UUID, in RoleInput) (Role, error) {
	in, _, err := validateRole(in)
	if err != nil {
		return Role{}, err
	}
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.store.role(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		if err != nil {
			return err
		}
		if cur.System {
			return ErrSystemRole
		}
		if err := s.mayGrant(ctx, tx, actor, in.Permissions); err != nil {
			return err
		}
		if err := s.store.updateRole(ctx, tx, id, in); err != nil {
			return err
		}
		if err := s.store.setPermissions(ctx, tx, id, in.Permissions); err != nil {
			return err
		}
		if cur.Team != in.Team {
			if err := s.store.syncTeam(ctx, tx, id); err != nil {
				return err
			}
			// Tokens carry the team (lanes, alerts), so members sign in again to pick it up.
			if err := s.store.revokeRoleSessions(ctx, tx, id); err != nil {
				return err
			}
		}
		return s.store.audit(ctx, tx, actor, "access.role_updated", "role", id.String(), auditRole(in))
	})
	if err != nil {
		return Role{}, err
	}
	s.forget()
	return s.store.role(ctx, s.pool, id)
}

func (s *Service) DeleteRole(ctx context.Context, actor, id uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.store.role(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		if err != nil {
			return err
		}
		if cur.System {
			return ErrSystemRole
		}
		if used, err := s.store.roleInUse(ctx, tx, id); err != nil || used {
			if err == nil {
				err = ErrRoleInUse
			}
			return err
		}
		if err := s.store.deleteRole(ctx, tx, id); err != nil {
			return err
		}
		return s.store.audit(ctx, tx, actor, "access.role_deleted", "role", id.String(), map[string]string{"name": cur.Name})
	})
}

// ---- people

func (s *Service) Members(ctx context.Context) ([]Member, error) { return s.store.members(ctx, s.pool) }

func (s *Service) CreateMember(ctx context.Context, actor uuid.UUID, m NewMember) (Member, error) {
	m.Email = strings.ToLower(strings.TrimSpace(m.Email))
	m.Name = strings.Join(strings.Fields(m.Name), " ")
	m.Phone = strings.TrimSpace(m.Phone)
	f := map[string]string{}
	if a, err := mail.ParseAddress(m.Email); err != nil || a.Address != m.Email {
		f["email"] = "must be a valid email address"
	}
	if n := len([]rune(m.Name)); n < nameMin || n > nameMax {
		f["name"] = "must be 2 to 60 characters"
	}
	if len(m.Password) < minPasswordLen {
		f["password"] = "must be at least 12 characters"
	}
	if len(f) > 0 {
		return Member{}, apperr.Validation(f)
	}
	hash, err := auth.HashPassword(m.Password)
	if err != nil {
		return Member{}, err
	}
	var id uuid.UUID
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		role, err := s.store.role(ctx, tx, m.RoleID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		if err != nil {
			return err
		}
		if role.Key == guestRoleKey {
			return ErrGuestRole
		}
		if err := s.mayAssign(ctx, tx, actor, role); err != nil {
			return err
		}
		if id, err = s.store.insertMember(ctx, tx, m, hash); err != nil {
			return err
		}
		return s.store.audit(ctx, tx, actor, "access.user_created", "user", id.String(), map[string]string{"email": m.Email, "role": role.Name})
	})
	if err != nil {
		return Member{}, err
	}
	return s.store.member(ctx, s.pool, id)
}

// UpdateMember edits an account. Deactivating it, changing its role or resetting its
// password signs the person out everywhere.
func (s *Service) UpdateMember(ctx context.Context, actor, id uuid.UUID, c MemberChange) (Member, error) {
	if c.Name != nil {
		n := strings.Join(strings.Fields(*c.Name), " ")
		if l := len([]rune(n)); l < nameMin || l > nameMax {
			return Member{}, apperr.Validation(map[string]string{"name": "must be 2 to 60 characters"})
		}
		c.Name = &n
	}
	var hash *string
	if c.Password != nil {
		if len(*c.Password) < minPasswordLen {
			return Member{}, apperr.Validation(map[string]string{"password": "must be at least 12 characters"})
		}
		h, err := auth.HashPassword(*c.Password)
		if err != nil {
			return Member{}, err
		}
		hash = &h
	}
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.store.member(ctx, tx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		if err != nil {
			return err
		}
		adminID, err := s.store.adminRoleID(ctx, tx)
		if err != nil {
			return err
		}
		if cur.RoleID == adminID {
			// Editing an Administrator (password, role, deactivation) is itself admin-only.
			if ok, err := s.store.isAdmin(ctx, tx, actor); err != nil || !ok {
				return cmp.Or[error](err, ErrAdminOnly)
			}
		}
		wasAdmin := cur.RoleID == adminID && cur.Active
		leavesAdmin := (c.RoleID != nil && *c.RoleID != adminID) || (c.Active != nil && !*c.Active)
		if leavesAdmin && id == actor {
			return ErrSelfLockout
		}
		if wasAdmin && leavesAdmin {
			if n, err := s.store.activeAdmins(ctx, tx); err != nil || n <= 1 {
				if err == nil {
					err = ErrLastAdmin
				}
				return err
			}
		}
		if c.RoleID != nil {
			role, err := s.store.role(ctx, tx, *c.RoleID)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrRoleNotFound
			}
			if err != nil {
				return err
			}
			if role.Key == guestRoleKey {
				return ErrGuestRole
			}
			if err := s.mayAssign(ctx, tx, actor, role); err != nil {
				return err
			}
		}
		if err := s.store.updateMember(ctx, tx, id, c, hash); err != nil {
			return err
		}
		signOut := (c.Active != nil && !*c.Active) || (c.RoleID != nil && *c.RoleID != cur.RoleID) || hash != nil
		if signOut {
			if err := s.store.revokeSessions(ctx, tx, id); err != nil {
				return err
			}
		}
		return s.store.audit(ctx, tx, actor, "access.user_updated", "user", id.String(), auditMember(c))
	})
	if err != nil {
		return Member{}, err
	}
	s.forget()
	return s.store.member(ctx, s.pool, id)
}

// ---- escalation guards: nobody hands out more than they hold

// mayAssign checks the actor may give someone this role: the Administrator role only from an
// Administrator, any other role only if the actor holds all of its permissions.
func (s *Service) mayAssign(ctx context.Context, q db.Querier, actor uuid.UUID, role Role) error {
	if role.Key == adminRoleKey {
		if ok, err := s.store.isAdmin(ctx, q, actor); err != nil || !ok {
			return cmp.Or[error](err, ErrAdminOnly)
		}
		return nil
	}
	return s.mayGrant(ctx, q, actor, role.Permissions)
}

// mayGrant checks the actor holds every permission they are handing out.
func (s *Service) mayGrant(ctx context.Context, q db.Querier, actor uuid.UUID, perms []auth.Permission) error {
	if ok, err := s.store.isAdmin(ctx, q, actor); err != nil || ok {
		return err
	}
	held, err := s.Permissions(ctx, actor)
	if err != nil {
		return err
	}
	for _, p := range perms {
		if !slices.Contains(held, p) {
			return ErrGrantBeyondOwn
		}
	}
	return nil
}

// ---- validation

var keySafe = regexp.MustCompile(`[^A-Z0-9]+`)

// validateRole tidies the input and derives the role's key from its name.
func validateRole(in RoleInput) (RoleInput, string, error) {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	in.Description = strings.TrimSpace(in.Description)
	f := map[string]string{}
	if n := len([]rune(in.Name)); n < nameMin || n > nameMax {
		f["name"] = "must be 2 to 60 characters"
	}
	if len([]rune(in.Description)) > descriptionMax {
		f["description"] = "must be 200 characters or fewer"
	}
	if !in.Team.Valid() || in.Team == domain.RoleGuest || in.Team == domain.RoleAdmin {
		f["team"] = "must be Reservations, Events & Sales or General Manager"
	}
	seen := map[auth.Permission]bool{}
	var perms []auth.Permission
	for _, p := range in.Permissions {
		if !auth.IsStaffPermission(p) {
			f["permissions"] = "contains a permission that doesn't exist"
			break
		}
		if !seen[p] {
			seen[p] = true
			perms = append(perms, p)
		}
	}
	in.Permissions = perms
	if len(f) > 0 {
		return in, "", apperr.Validation(f)
	}
	key := strings.Trim(keySafe.ReplaceAllString(strings.ToUpper(in.Name), "_"), "_")
	if key == "" || key[0] < 'A' || key[0] > 'Z' {
		key = "ROLE_" + key
	}
	if len(key) > 40 {
		key = key[:40]
	}
	return in, key, nil
}

func auditRole(in RoleInput) []byte {
	b, _ := json.Marshal(map[string]any{"name": in.Name, "team": in.Team, "permissions": in.Permissions})
	return b
}

func auditMember(c MemberChange) []byte {
	b, _ := json.Marshal(map[string]any{"name": c.Name, "role_id": c.RoleID, "active": c.Active, "password_reset": c.Password != nil})
	return b
}

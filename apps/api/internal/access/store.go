package access

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
)

// store is every query this package runs.
type store struct{}

// grants returns the user's role key and its permissions; an inactive or unknown user
// gets pgx.ErrNoRows.
func (store) grants(ctx context.Context, q db.Querier, userID uuid.UUID) (string, []auth.Permission, error) {
	var key string
	var perms []string
	err := q.QueryRow(ctx, `SELECT r.key, coalesce(array_agg(rp.permission) FILTER (WHERE rp.permission IS NOT NULL), '{}')
		FROM users u JOIN roles r ON r.id = u.role_id LEFT JOIN role_permissions rp ON rp.role_id = r.id
		WHERE u.id = $1 AND u.is_active GROUP BY r.key`, userID).Scan(&key, &perms)
	out := make([]auth.Permission, len(perms))
	for i, p := range perms {
		out[i] = auth.Permission(p)
	}
	return key, out, err
}

func (store) catalogue(ctx context.Context, q db.Querier) ([]PermissionInfo, error) {
	rows, err := q.Query(ctx, `SELECT key, grp, label, description FROM permissions ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (PermissionInfo, error) {
		var p PermissionInfo
		return p, r.Scan(&p.Key, &p.Group, &p.Label, &p.Description)
	})
}

const roleSelect = `SELECT r.id, r.key, r.name, r.description, r.team, r.is_system,
		coalesce((SELECT array_agg(permission ORDER BY permission) FROM role_permissions WHERE role_id = r.id), '{}'),
		(SELECT count(*) FROM users WHERE role_id = r.id AND is_active)
	FROM roles r`

func scanRole(r pgx.CollectableRow) (Role, error) {
	var x Role
	var perms []string
	if err := r.Scan(&x.ID, &x.Key, &x.Name, &x.Description, &x.Team, &x.System, &perms, &x.Members); err != nil {
		return x, err
	}
	for _, p := range perms {
		x.Permissions = append(x.Permissions, auth.Permission(p))
	}
	return x, nil
}

func (store) roles(ctx context.Context, q db.Querier) ([]Role, error) {
	rows, err := q.Query(ctx, roleSelect+` ORDER BY r.is_system DESC, r.name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRole)
}

// role locks the row when q is a transaction; pgx.ErrNoRows if it doesn't exist.
func (store) role(ctx context.Context, q db.Querier, id uuid.UUID) (Role, error) {
	rows, err := q.Query(ctx, roleSelect+` WHERE r.id = $1`, id)
	if err != nil {
		return Role{}, err
	}
	return pgx.CollectExactlyOneRow(rows, scanRole)
}

func (store) insertRole(ctx context.Context, q db.Querier, key string, in RoleInput) (uuid.UUID, error) {
	id := idgen.New()
	_, err := q.Exec(ctx, `INSERT INTO roles (id, key, name, description, team) VALUES ($1, $2, $3, $4, $5)`,
		id, key, in.Name, in.Description, in.Team)
	if isUnique(err) {
		return id, ErrRoleExists
	}
	return id, err
}

func (store) updateRole(ctx context.Context, q db.Querier, id uuid.UUID, in RoleInput) error {
	_, err := q.Exec(ctx, `UPDATE roles SET name = $2, description = $3, team = $4, updated_at = now() WHERE id = $1`,
		id, in.Name, in.Description, in.Team)
	return err
}

// setPermissions replaces a role's grants. Members' teams follow the role's team.
func (store) setPermissions(ctx context.Context, q db.Querier, id uuid.UUID, perms []auth.Permission) error {
	if _, err := q.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, id); err != nil {
		return err
	}
	keys := make([]string, len(perms))
	for i, p := range perms {
		keys[i] = string(p)
	}
	_, err := q.Exec(ctx, `INSERT INTO role_permissions (role_id, permission) SELECT $1, unnest($2::text[])`, id, keys)
	return err
}

// syncTeam re-applies the role's team to its members (the users trigger copies it).
func (store) syncTeam(ctx context.Context, q db.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE users SET role_id = role_id, role = (SELECT team FROM roles WHERE id = $1), updated_at = now()
		WHERE role_id = $1`, id)
	return err
}

func (store) deleteRole(ctx context.Context, q db.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
	return err
}

func (store) roleInUse(ctx context.Context, q db.Querier, id uuid.UUID) (bool, error) {
	var used bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE role_id = $1)`, id).Scan(&used)
	return used, err
}

const memberSelect = `SELECT u.id, u.email, u.name, u.phone, u.role_id, r.name, r.team, u.is_active, u.created_at
	FROM users u JOIN roles r ON r.id = u.role_id`

func scanMember(r pgx.CollectableRow) (Member, error) {
	var m Member
	return m, r.Scan(&m.ID, &m.Email, &m.Name, &m.Phone, &m.RoleID, &m.RoleName, &m.Team, &m.Active, &m.CreatedAt)
}

func (store) members(ctx context.Context, q db.Querier) ([]Member, error) {
	rows, err := q.Query(ctx, memberSelect+` WHERE r.key <> $1 ORDER BY u.is_active DESC, u.name`, guestRoleKey)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMember)
}

// member locks the account when q is a transaction; pgx.ErrNoRows for guests and unknown ids.
func (store) member(ctx context.Context, q db.Querier, id uuid.UUID) (Member, error) {
	rows, err := q.Query(ctx, memberSelect+` WHERE u.id = $1 AND r.key <> $2 FOR UPDATE OF u`, id, guestRoleKey)
	if err != nil {
		return Member{}, err
	}
	return pgx.CollectExactlyOneRow(rows, scanMember)
}

func (store) insertMember(ctx context.Context, q db.Querier, m NewMember, hash string) (uuid.UUID, error) {
	id := idgen.New()
	_, err := q.Exec(ctx, `INSERT INTO users (id, email, name, phone, role, role_id, password_hash)
		VALUES ($1, $2, $3, $4, (SELECT team FROM roles WHERE id = $5), $5, $6)`,
		id, m.Email, m.Name, nilIfEmpty(m.Phone), m.RoleID, hash)
	if isUnique(err) {
		return id, ErrEmailTaken
	}
	return id, err
}

func (store) updateMember(ctx context.Context, q db.Querier, id uuid.UUID, c MemberChange, hash *string) error {
	_, err := q.Exec(ctx, `UPDATE users SET name = coalesce($2, name), phone = coalesce($3, phone), role_id = coalesce($4, role_id),
		is_active = coalesce($5, is_active), password_hash = coalesce($6, password_hash), updated_at = now() WHERE id = $1`,
		id, c.Name, c.Phone, c.RoleID, c.Active, hash)
	return err
}

// activeAdmins counts active accounts on the Administrator role, locking them so two
// administrators can't demote each other at once.
func (store) activeAdmins(ctx context.Context, q db.Querier) (int, error) {
	rows, err := q.Query(ctx, `SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id
		WHERE r.key = $1 AND u.is_active FOR UPDATE OF u`, adminRoleKey)
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return len(ids), err
}

func (store) adminRoleID(ctx context.Context, q db.Querier) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT id FROM roles WHERE key = $1`, adminRoleKey).Scan(&id)
	return id, err
}

// revokeSessions signs a person out everywhere.
func (store) revokeSessions(ctx context.Context, q db.Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (store) audit(ctx context.Context, q db.Querier, actor uuid.UUID, action, entity, entityID string, after any) error {
	_, err := q.Exec(ctx, `INSERT INTO audit_logs (id, actor_user_id, action, entity, entity_id, after) VALUES ($1, $2, $3, $4, $5, $6)`,
		idgen.New(), actor, action, entity, entityID, after)
	return err
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

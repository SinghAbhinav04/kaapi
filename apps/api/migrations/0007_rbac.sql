-- +goose Up
-- Role-based access in the database. The administrator creates roles in the console, picks
-- exactly what each can see and do, and assigns them to the staff accounts they create.
--   permissions       the catalogue (keys match internal/auth/permissions.go; a test checks)
--   roles             a name, a description and the team the role works for
--   role_permissions  what each role grants
-- A role's team (users.role) still decides whose lane an enquiry is assigned to, who gets
-- complaint alerts and the signature title; permissions decide everything a person can see.

CREATE TABLE permissions (
    key         text PRIMARY KEY,
    grp         text NOT NULL,   -- heading in the console, e.g. "Inbox"
    label       text NOT NULL,
    description text NOT NULL,
    sort        int  NOT NULL
);

CREATE TABLE roles (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    key         text        NOT NULL UNIQUE CHECK (key ~ '^[A-Z][A-Z0-9_]{1,39}$'),
    name        text        NOT NULL CHECK (length(name) BETWEEN 2 AND 60),
    description text        NOT NULL DEFAULT '',
    team        text        NOT NULL CHECK (team IN ('GUEST', 'RESERVATIONS', 'SALES', 'GM', 'ADMIN')),
    is_system   boolean     NOT NULL DEFAULT false,  -- Administrator and Guest: can't be edited or deleted
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission text NOT NULL REFERENCES permissions (key) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission)
);

ALTER TABLE permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_permissions ENABLE ROW LEVEL SECURITY;

INSERT INTO permissions (key, grp, label, description, sort) VALUES
    ('enquiries:read',           'Inbox',     'Use the inbox',              'Read enquiries in their team''s lane, edit drafts, change status.', 10),
    ('enquiries:read_all_lanes', 'Inbox',     'See every lane',             'Every team''s enquiries, including complaints for the GM.', 20),
    ('enquiries:reroute',        'Inbox',     'Move enquiries',             'Send an enquiry to another lane.', 30),
    ('enquiries:send',           'Inbox',     'Send replies and messages',  'Send replies to guests and write new messages.', 40),
    ('bookings:read',            'Bookings',  'See bookings',               'The bookings list, including the archive.', 50),
    ('bookings:export',          'Bookings',  'Export bookings',            'Download the daily CSV for the reservation system.', 60),
    ('rates:read',               'Rates',     'See rates and allotment',    'The rates and allotment grid.', 70),
    ('rates:write',              'Rates',     'Change prices',              'Set nightly prices.', 80),
    ('inventory:write',          'Rates',     'Change allotment',           'Allotment, stop-sell, minimum stay and arrival rules.', 90),
    ('promos:write',             'Rates',     'Manage offers',              'Create and change promo codes.', 100),
    ('outbox:read',              'Messages',  'See the outbox',             'Every message the system has sent.', 110),
    ('facts:write',              'Messages',  'Edit the fact sheet',        'The resort facts the AI uses in drafts.', 120),
    ('dashboard:read',           'Reports',   'See the dashboard',          'This week''s success measures.', 130),
    ('analytics:read',           'Reports',   'See analytics',              'Revenue, booking pace and occupancy.', 140),
    ('integrations:manage',      'Admin',     'Manage the mailbox',         'Connect or disconnect Gmail.', 150),
    ('retention:run',            'Admin',     'Run data retention',         'Archive and delete data on demand.', 160),
    ('users:manage',             'Admin',     'Manage team and access',     'Create staff accounts, roles and what each can see.', 170),
    ('own_bookings:read',        'Guest',     'Own bookings',               'A guest''s own bookings (guest accounts only).', 999);

INSERT INTO roles (key, name, description, team, is_system) VALUES
    ('ADMIN',        'Administrator',   'Everything, including team and access. Can''t be changed.', 'ADMIN', true),
    ('GM',           'General Manager', 'Every lane, rates, offers, reports.', 'GM', false),
    ('RESERVATIONS', 'Reservations',    'Room enquiries, bookings, rates and allotment.', 'RESERVATIONS', false),
    ('SALES',        'Events & Sales',  'Wedding and offsite enquiries.', 'SALES', false),
    ('GUEST',        'Guest',           'Website guests signed in by email code.', 'GUEST', true);

-- Built-in grants: exactly what the code granted before this migration.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.perm FROM roles r
JOIN (VALUES
    ('GUEST', 'own_bookings:read'),
    ('SALES', 'bookings:read'), ('SALES', 'rates:read'), ('SALES', 'enquiries:read'), ('SALES', 'enquiries:reroute'),
    ('SALES', 'enquiries:send'), ('SALES', 'outbox:read'), ('SALES', 'dashboard:read'),
    ('RESERVATIONS', 'bookings:read'), ('RESERVATIONS', 'rates:read'), ('RESERVATIONS', 'enquiries:read'),
    ('RESERVATIONS', 'enquiries:reroute'), ('RESERVATIONS', 'enquiries:send'), ('RESERVATIONS', 'outbox:read'),
    ('RESERVATIONS', 'dashboard:read'), ('RESERVATIONS', 'rates:write'), ('RESERVATIONS', 'inventory:write'),
    ('RESERVATIONS', 'bookings:export'),
    ('GM', 'bookings:read'), ('GM', 'rates:read'), ('GM', 'enquiries:read'), ('GM', 'enquiries:reroute'),
    ('GM', 'enquiries:send'), ('GM', 'outbox:read'), ('GM', 'dashboard:read'), ('GM', 'rates:write'),
    ('GM', 'inventory:write'), ('GM', 'promos:write'), ('GM', 'enquiries:read_all_lanes'), ('GM', 'bookings:export'),
    ('GM', 'facts:write'), ('GM', 'analytics:read'), ('GM', 'integrations:manage')
) AS p(role, perm) ON p.role = r.key;
-- ADMIN needs no rows: the code grants the Administrator every permission, including future ones.

ALTER TABLE users ADD COLUMN role_id uuid REFERENCES roles (id);
UPDATE users u SET role_id = r.id FROM roles r WHERE r.key = u.role;
ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;
CREATE INDEX users_role_idx ON users (role_id);

-- Keep a user's team and role in step, whichever one a statement sets: setting role_id copies
-- the role's team into users.role; setting only users.role (older code paths, the seeder,
-- guest sign-up) picks the built-in role of that name.
-- +goose StatementBegin
CREATE FUNCTION sync_user_role() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.role_id IS NULL
       OR TG_OP = 'UPDATE' AND NEW.role_id IS NOT DISTINCT FROM OLD.role_id AND NEW.role IS DISTINCT FROM OLD.role THEN
        SELECT id INTO NEW.role_id FROM roles WHERE key = NEW.role;
    ELSE
        SELECT team INTO NEW.role FROM roles WHERE id = NEW.role_id;
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER users_sync_role BEFORE INSERT OR UPDATE OF role, role_id ON users
    FOR EACH ROW EXECUTE FUNCTION sync_user_role();

-- +goose Down
DROP TRIGGER IF EXISTS users_sync_role ON users;
DROP FUNCTION IF EXISTS sync_user_role();
ALTER TABLE users DROP COLUMN IF EXISTS role_id;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS permissions;

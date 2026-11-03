-- +goose Up
-- Retention (see internal/retention). A finished booking is archived rather than moved to
-- another table, so its nights, payments and audit trail stay linked for the tax window.
ALTER TABLE bookings
    ADD COLUMN archived_at   timestamptz,
    ADD COLUMN anonymised_at timestamptz;

-- The staff list shows live bookings by default and the archive on request.
CREATE INDEX bookings_archived_idx ON bookings (archived_at) WHERE archived_at IS NOT NULL;

-- The sweep's lookups by age.
CREATE INDEX enquiries_created_idx ON enquiries (created_at);
CREATE INDEX outbox_created_idx ON outbox_messages (created_at);

-- +goose Down
DROP INDEX IF EXISTS outbox_created_idx;
DROP INDEX IF EXISTS enquiries_created_idx;
DROP INDEX IF EXISTS bookings_archived_idx;
ALTER TABLE bookings DROP COLUMN IF EXISTS anonymised_at, DROP COLUMN IF EXISTS archived_at;

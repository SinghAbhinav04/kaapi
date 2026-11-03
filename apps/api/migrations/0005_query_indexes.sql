-- +goose NO TRANSACTION
-- Indexes chosen by measuring the app's real queries with EXPLAIN ANALYZE on a database
-- loaded with ~3 years of traffic (200k bookings, 300k enquiries, 900k events, 400k holds).
-- Built CONCURRENTLY so applying this to a live database never blocks writes; that needs
-- each statement outside a transaction, hence NO TRANSACTION and IF NOT EXISTS (re-runnable).

-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Inbox: open enquiries newest first, the urgent ones (HIGH + NEW) fetched separately so
-- neither part needs a sort over every open row. 200 ms → 5-11 ms.
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_open_idx ON enquiries (received_at DESC)
    WHERE status <> ALL ('{CLOSED,SPAM}'::text[]);
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_open_lane_idx ON enquiries (lane, received_at DESC)
    WHERE status <> ALL ('{CLOSED,SPAM}'::text[]);
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_urgent_idx ON enquiries (received_at DESC)
    WHERE priority = 'HIGH' AND status = 'NEW';
-- Inbox search: one trigram index over the searched text. 215 ms → 6 ms.
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_search_trgm_idx ON enquiries
    USING gin ((coalesce(sender_name, '') || ' ' || ref || ' ' || raw_text) gin_trgm_ops);
-- The AI worker's queue head, polled every 15 s and on every new message. Seq scan → 0.05 ms.
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_pending_idx ON enquiries (received_at) WHERE ai_status = 'PENDING';
-- Intake's per-sender hourly cap and dedupe. 24 ms → 0.1 ms on every message received.
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_sender_recent_idx ON enquiries ((coalesce(sender_email, sender_phone)), received_at DESC);
-- Dashboard and analytics date ranges.
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_received_idx ON enquiries (received_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_first_response_idx ON enquiries (first_response_at) WHERE first_response_at IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiry_events_created_idx ON enquiry_events (created_at);

-- Staff bookings: live list in stay order, archive newest first. 70-150 ms → 3-4 ms.
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_live_idx ON bookings (check_in, created_at) WHERE archived_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_archive_list_idx ON bookings (check_in DESC, created_at DESC) WHERE archived_at IS NOT NULL;
-- Retention's "stay is over, not archived yet" pick.
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_to_archive_idx ON bookings (check_out) WHERE archived_at IS NULL;
-- Analytics (booked date), dashboard and the daily export (confirmed/cancelled date).
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_created_idx ON bookings (created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_confirmed_idx ON bookings (confirmed_at) WHERE confirmed_at IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_cancelled_idx ON bookings (cancelled_at) WHERE cancelled_at IS NOT NULL;
-- "My bookings" matches the guest's account OR email; both sides need an index. 26 ms → 1 ms.
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_guest_user_idx ON bookings (guest_user_id) WHERE guest_user_id IS NOT NULL;
-- On-the-books by stay date across all rooms (the existing index leads with room type).
CREATE INDEX CONCURRENTLY IF NOT EXISTS booking_nights_date_idx ON booking_nights (date);

-- Replaced or duplicated.
DROP INDEX CONCURRENTLY IF EXISTS bookings_archived_idx;   -- by bookings_archive_list_idx, which also gives the order
DROP INDEX CONCURRENTLY IF EXISTS outbox_created_idx;      -- duplicate of outbox_messages_created_idx (0001)

-- A re-seed truncates users with CASCADE, which would also wipe the Gmail connection
-- through this reference. It's only an audit note, so keep the id without the constraint.
ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_connected_by_fkey;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS enquiries_open_idx;
DROP INDEX CONCURRENTLY IF EXISTS enquiries_open_lane_idx;
DROP INDEX CONCURRENTLY IF EXISTS enquiries_urgent_idx;
DROP INDEX CONCURRENTLY IF EXISTS enquiries_search_trgm_idx;
DROP INDEX CONCURRENTLY IF EXISTS enquiries_pending_idx;
DROP INDEX CONCURRENTLY IF EXISTS enquiries_sender_recent_idx;
DROP INDEX CONCURRENTLY IF EXISTS enquiries_received_idx;
DROP INDEX CONCURRENTLY IF EXISTS enquiries_first_response_idx;
DROP INDEX CONCURRENTLY IF EXISTS enquiry_events_created_idx;
DROP INDEX CONCURRENTLY IF EXISTS bookings_live_idx;
DROP INDEX CONCURRENTLY IF EXISTS bookings_archive_list_idx;
DROP INDEX CONCURRENTLY IF EXISTS bookings_to_archive_idx;
DROP INDEX CONCURRENTLY IF EXISTS bookings_created_idx;
DROP INDEX CONCURRENTLY IF EXISTS bookings_confirmed_idx;
DROP INDEX CONCURRENTLY IF EXISTS bookings_cancelled_idx;
DROP INDEX CONCURRENTLY IF EXISTS bookings_guest_user_idx;
DROP INDEX CONCURRENTLY IF EXISTS booking_nights_date_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_archived_idx ON bookings (archived_at) WHERE archived_at IS NOT NULL;

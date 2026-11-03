-- +goose NO TRANSACTION
-- Personal data encrypted at rest (DPDP). The API stores these as AES-256-GCM ciphertext
-- (internal/platform/pii); the database never sees them in the clear:
--   bookings  guest_name, guest_email, guest_phone, guest_country
--   enquiries sender_name, sender_email, sender_phone
--   users     email, name, phone
--   outbox_messages recipient, body
-- Ciphertext can't be compared, so equality lookups move to keyed hashes ("blind indexes"):
--   bookings.guest_email_hash  a guest's own bookings
--   enquiries.sender_hash      intake's dedupe and per-sender cap (email, else phone)
--   users.email_hash           sign-in, and one account per email (unique)
--   otp_codes.email_hash       codes are only ever matched, so the email isn't kept at all
-- Existing rows are encrypted by the API's backfill after this runs. Index changes are
-- CONCURRENT so this never blocks writes on a live database.

-- +goose Up
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS guest_email_hash text;
-- The country is ciphertext now; the API validates it before encrypting.
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_guest_country_check;
ALTER TABLE enquiries ADD COLUMN IF NOT EXISTS sender_hash text;

DROP INDEX CONCURRENTLY IF EXISTS bookings_guest_email_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_guest_email_hash_idx ON bookings (guest_email_hash);

DROP INDEX CONCURRENTLY IF EXISTS enquiries_sender_recent_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_sender_hash_recent_idx ON enquiries (sender_hash, received_at DESC);

ALTER TABLE users ADD COLUMN IF NOT EXISTS email_hash text;
DROP INDEX CONCURRENTLY IF EXISTS users_email_key;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS users_email_hash_key ON users (email_hash);

-- Pending codes live five minutes; clearing them is simpler than hashing them in SQL.
DELETE FROM otp_codes;
ALTER TABLE otp_codes RENAME COLUMN email TO email_hash;
DROP INDEX CONCURRENTLY IF EXISTS otp_codes_email_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS otp_codes_email_hash_idx ON otp_codes (email_hash, created_at DESC);

-- Inbox search can't read encrypted sender names; it searches the reference and the message.
DROP INDEX CONCURRENTLY IF EXISTS enquiries_search_trgm_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_search_trgm_idx ON enquiries
    USING gin ((ref || ' ' || raw_text) gin_trgm_ops);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS otp_codes_email_hash_idx;
DELETE FROM otp_codes;
ALTER TABLE otp_codes RENAME COLUMN email_hash TO email;
CREATE INDEX CONCURRENTLY IF NOT EXISTS otp_codes_email_idx ON otp_codes (lower(email), created_at DESC);
DROP INDEX CONCURRENTLY IF EXISTS users_email_hash_key;
ALTER TABLE users DROP COLUMN IF EXISTS email_hash;
-- users_email_key on lower(email) can't come back while emails are ciphertext; decrypt first.
DROP INDEX CONCURRENTLY IF EXISTS enquiries_search_trgm_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_search_trgm_idx ON enquiries
    USING gin ((coalesce(sender_name, '') || ' ' || ref || ' ' || raw_text) gin_trgm_ops);
DROP INDEX CONCURRENTLY IF EXISTS enquiries_sender_hash_recent_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_sender_recent_idx ON enquiries ((coalesce(sender_email, sender_phone)), received_at DESC);
DROP INDEX CONCURRENTLY IF EXISTS bookings_guest_email_hash_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_guest_email_idx ON bookings (lower(guest_email));
ALTER TABLE enquiries DROP COLUMN IF EXISTS sender_hash;
ALTER TABLE bookings DROP COLUMN IF EXISTS guest_email_hash;
-- The country check isn't restored: ciphertext rows would fail it until decrypted.

-- +goose NO TRANSACTION
-- Analytics aggregates bookings by booked date inside Postgres and returns only totals.
-- With every column those aggregates read carried in the index, Postgres answers from the
-- index alone (index-only scan) instead of fetching each booking row: a 12-month report over
-- ~3 years of bookings went from ~295 ms to ~45 ms in the benchmark. It replaces the plain
-- created_at index from 0005.

-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_created_cover_idx ON bookings (created_at)
    INCLUDE (status, check_in, check_out, nights, rooms, total_paise, subtotal_paise, discount_paise, tax_paise, room_type_id, rate_plan_id);
DROP INDEX CONCURRENTLY IF EXISTS bookings_created_idx;

-- +goose Down
CREATE INDEX CONCURRENTLY IF NOT EXISTS bookings_created_idx ON bookings (created_at);
DROP INDEX CONCURRENTLY IF EXISTS bookings_created_cover_idx;

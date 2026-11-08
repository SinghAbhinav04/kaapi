-- +goose NO TRANSACTION
-- +goose Up
-- Every imported email asks "does this thread already belong to an enquiry?" (migration 0018).
CREATE INDEX CONCURRENTLY IF NOT EXISTS enquiries_thread_idx ON enquiries (thread_id) WHERE thread_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS enquiries_thread_idx;

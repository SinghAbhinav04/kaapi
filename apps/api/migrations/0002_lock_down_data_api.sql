-- +goose Up
-- On Supabase, every table in `public` is reachable through the auto-generated REST API
-- with the project's publishable key, which is public by design. Row-level security with
-- no policies denies those roles everything. The Go API connects as the tables' owner,
-- which RLS does not apply to, so the app is unaffected. On plain Postgres this is
-- harmless, and it keeps the database safe if it is ever exposed the same way.
-- +goose StatementBegin
DO $$
DECLARE t record;
BEGIN
    FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = current_schema() LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t.tablename);
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE t record;
BEGIN
    FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = current_schema() LOOP
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t.tablename);
    END LOOP;
END $$;
-- +goose StatementEnd

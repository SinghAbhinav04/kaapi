-- +goose Up
-- The inbox updates live: every change to an enquiry or its drafts sends the enquiry's ref
-- on the 'inbox_changes' channel, whichever process made it (the API or the worker). The API
-- listens once and pushes it to open inbox pages (server-sent events). Postgres delivers
-- NOTIFY only on commit, and folds identical payloads within one transaction.
-- +goose StatementBegin
CREATE FUNCTION notify_inbox() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
BEGIN
    IF TG_TABLE_NAME = 'enquiries' THEN
        PERFORM pg_notify('inbox_changes', NEW.ref);
    ELSE
        PERFORM pg_notify('inbox_changes', (SELECT ref FROM enquiries WHERE id = NEW.enquiry_id));
    END IF;
    RETURN NULL;
END $$;
-- +goose StatementEnd

CREATE TRIGGER enquiries_notify AFTER INSERT OR UPDATE ON enquiries
    FOR EACH ROW EXECUTE FUNCTION notify_inbox();
CREATE TRIGGER enquiry_drafts_notify AFTER INSERT OR UPDATE ON enquiry_drafts
    FOR EACH ROW EXECUTE FUNCTION notify_inbox();

-- +goose Down
DROP TRIGGER IF EXISTS enquiry_drafts_notify ON enquiry_drafts;
DROP TRIGGER IF EXISTS enquiries_notify ON enquiries;
DROP FUNCTION IF EXISTS notify_inbox();

-- +goose Up
-- Replies are now signed by whoever sends them (enquiry.Send adds the sign-off), not by the
-- lane's first staff member when the draft was written. Unsent drafts written before this
-- still end with that old sign-off ("Rohan\nReservations, Kaapi Estate"), which would sign
-- twice; strip it. Sent drafts keep it: that is what the guest received.
UPDATE enquiry_drafts
SET body_rendered = regexp_replace(body_rendered, E'\\n\\n[^\\n]+\\n[^\\n]*Kaapi Estate[ \\t]*$', '')
WHERE sent_at IS NULL AND body_rendered ~ E'\\n\\n[^\\n]+\\n[^\\n]*Kaapi Estate[ \\t]*$';

-- The console's alerts bell refreshes when an alert arrives or is read, from any process, the
-- same way the other live views do (migrations 0013 and 0014).
CREATE TRIGGER notifications_data_change_ins AFTER INSERT ON notifications REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('notifications');
CREATE TRIGGER notifications_data_change_upd AFTER UPDATE ON notifications REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('notifications');

-- +goose Down
DROP TRIGGER IF EXISTS notifications_data_change_ins ON notifications;
DROP TRIGGER IF EXISTS notifications_data_change_upd ON notifications;

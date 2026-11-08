-- +goose Up
-- A guest who answers our reply writes in the same Gmail thread. That used to arrive as a brand
-- new enquiry (new reference, new AI draft, the history split in two); now it joins the
-- enquiry the thread belongs to as a follow-up message and reopens it.

-- The guest's later messages on an enquiry, in order. The text stays plain for the same reason
-- enquiries.raw_text does (staff read and search it; it's redacted before any model call);
-- they go when their enquiry goes (retention: 12 months).
CREATE TABLE enquiry_messages (
    id             uuid        PRIMARY KEY,
    enquiry_id     uuid        NOT NULL REFERENCES enquiries (id) ON DELETE CASCADE,
    body           text        NOT NULL,
    external_id    text        UNIQUE,   -- the Gmail message id: polling twice stores it once
    rfc_message_id text,
    received_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX enquiry_messages_enquiry_idx ON enquiry_messages (enquiry_id, received_at);
ALTER TABLE enquiry_messages ENABLE ROW LEVEL SECURITY;

-- The lane's team hears that a guest replied.
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('COMPLAINT_ALERT', 'UNSORTED', 'BOOKING', 'REFUND_NEEDED', 'GUEST_REPLY'));

-- +goose Down
DELETE FROM notifications WHERE type = 'GUEST_REPLY';
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('COMPLAINT_ALERT', 'UNSORTED', 'BOOKING', 'REFUND_NEEDED'));
DROP TABLE enquiry_messages;

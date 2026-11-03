-- +goose Up
-- One connected mailbox per provider. The refresh token is encrypted by the API
-- (AES-256-GCM, key derived from JWE_SECRET) before it reaches the database.
CREATE TABLE integrations (
    provider          text        PRIMARY KEY CHECK (provider IN ('GMAIL')),
    account_email     text        NOT NULL,
    refresh_token_enc bytea       NOT NULL,
    history_id        text,                       -- where the next poll starts
    connected_by      uuid        REFERENCES users (id) ON DELETE SET NULL,
    connected_at      timestamptz NOT NULL DEFAULT now(),
    last_sync_at      timestamptz,
    last_error        text
);
ALTER TABLE integrations ENABLE ROW LEVEL SECURITY;

-- Where an emailed enquiry came from, so it's imported once and replies thread under it.
ALTER TABLE enquiries
    ADD COLUMN external_id    text UNIQUE,   -- Gmail message id
    ADD COLUMN thread_id      text,          -- Gmail thread id
    ADD COLUMN rfc_message_id text;          -- the Message-ID header, for In-Reply-To

-- A draft being delivered: claimed before the network call so a double click can't send twice.
ALTER TABLE enquiry_drafts ADD COLUMN sending_at timestamptz;

-- Real deliveries carry the provider's id; staff can also write a new message from scratch.
ALTER TABLE outbox_messages ADD COLUMN external_id text;
ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_kind_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_kind_check
    CHECK (kind IN ('OTP', 'BOOKING_CONFIRMATION', 'ENQUIRY_REPLY', 'ALERT', 'REFUND_NOTICE', 'STAFF_MESSAGE'));

-- +goose Down
ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_kind_check;
DELETE FROM outbox_messages WHERE kind = 'STAFF_MESSAGE';
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_kind_check
    CHECK (kind IN ('OTP', 'BOOKING_CONFIRMATION', 'ENQUIRY_REPLY', 'ALERT', 'REFUND_NOTICE'));
ALTER TABLE outbox_messages DROP COLUMN IF EXISTS external_id;
ALTER TABLE enquiry_drafts DROP COLUMN IF EXISTS sending_at;
ALTER TABLE enquiries DROP COLUMN IF EXISTS rfc_message_id, DROP COLUMN IF EXISTS thread_id, DROP COLUMN IF EXISTS external_id;
DROP TABLE IF EXISTS integrations;

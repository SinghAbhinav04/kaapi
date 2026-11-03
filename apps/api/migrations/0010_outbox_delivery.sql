-- +goose Up
-- Guest emails (sign-in codes, welcome, confirmations, refund notices) are written as QUEUED
-- in the same transaction as what caused them, so none is lost, and the worker delivers them
-- through the connected mailbox afterwards: claim, send with no transaction open, record.
ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_status_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_status_check
    CHECK (status IN ('QUEUED', 'SIMULATED', 'SENT', 'FAILED'));
ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_kind_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_kind_check
    CHECK (kind IN ('OTP', 'WELCOME', 'BOOKING_CONFIRMATION', 'ENQUIRY_REPLY', 'ALERT', 'REFUND_NOTICE', 'STAFF_MESSAGE'));

ALTER TABLE outbox_messages
    ADD COLUMN attempts        int         NOT NULL DEFAULT 0,
    ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN claimed_at      timestamptz,          -- set while a worker is sending it
    ADD COLUMN sent_at         timestamptz,
    ADD COLUMN last_error      text;

-- The worker's every-few-seconds lookup touches only what's waiting.
CREATE INDEX outbox_queued_idx ON outbox_messages (next_attempt_at) WHERE status = 'QUEUED';

-- +goose Down
DROP INDEX IF EXISTS outbox_queued_idx;
ALTER TABLE outbox_messages DROP COLUMN IF EXISTS last_error, DROP COLUMN IF EXISTS sent_at,
    DROP COLUMN IF EXISTS claimed_at, DROP COLUMN IF EXISTS next_attempt_at, DROP COLUMN IF EXISTS attempts;
UPDATE outbox_messages SET status = 'SIMULATED' WHERE status = 'QUEUED';
DELETE FROM outbox_messages WHERE kind = 'WELCOME';
ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_kind_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_kind_check
    CHECK (kind IN ('OTP', 'BOOKING_CONFIRMATION', 'ENQUIRY_REPLY', 'ALERT', 'REFUND_NOTICE', 'STAFF_MESSAGE'));
ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_status_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_status_check
    CHECK (status IN ('SIMULATED', 'SENT', 'FAILED'));

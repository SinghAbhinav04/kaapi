-- +goose Up
-- Guests get a reminder before they arrive and on the morning they leave; staff sign in with
-- a password and then a code emailed to them.

ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_kind_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_kind_check CHECK (kind IN (
    'OTP', 'WELCOME', 'BOOKING_CONFIRMATION', 'ENQUIRY_REPLY', 'ALERT', 'REFUND_NOTICE', 'STAFF_MESSAGE',
    'STAFF_CODE', 'CHECKIN_REMINDER', 'CHECKOUT_REMINDER'));

ALTER TABLE otp_codes DROP CONSTRAINT otp_codes_purpose_check;
ALTER TABLE otp_codes ADD CONSTRAINT otp_codes_purpose_check CHECK (purpose IN ('GUEST_LOGIN', 'STAFF_2FA'));

-- One row per reminder sent: the primary key is what makes each reminder go out exactly once,
-- however often (or in however many workers) the job runs.
CREATE TABLE booking_reminders (
    booking_id uuid        NOT NULL REFERENCES bookings (id) ON DELETE CASCADE,
    kind       text        NOT NULL CHECK (kind IN ('CHECKIN', 'CHECKOUT')),
    queued_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (booking_id, kind)
);
ALTER TABLE booking_reminders ENABLE ROW LEVEL SECURITY;

-- +goose Down
DROP TABLE IF EXISTS booking_reminders;
DELETE FROM otp_codes WHERE purpose = 'STAFF_2FA';
ALTER TABLE otp_codes DROP CONSTRAINT otp_codes_purpose_check;
ALTER TABLE otp_codes ADD CONSTRAINT otp_codes_purpose_check CHECK (purpose = 'GUEST_LOGIN');
DELETE FROM outbox_messages WHERE kind IN ('STAFF_CODE', 'CHECKIN_REMINDER', 'CHECKOUT_REMINDER');
ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_kind_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_kind_check CHECK (kind IN (
    'OTP', 'WELCOME', 'BOOKING_CONFIRMATION', 'ENQUIRY_REPLY', 'ALERT', 'REFUND_NOTICE', 'STAFF_MESSAGE'));

-- +goose Up
-- Offers and seasonal emails for guests who asked for them (the opt-in at checkout). Every
-- email is stored in the outbox like any other; campaign_recipients makes each campaign
-- reach each guest at most once, however the fan-out is interrupted or repeated.

ALTER TABLE users
    ADD COLUMN marketing_opt_in    boolean NOT NULL DEFAULT false,
    ADD COLUMN marketing_opt_in_at timestamptz,
    ADD COLUMN unsubscribe_token   text UNIQUE;  -- in every promotional email; only ever unsubscribes

-- Guests who already ticked the box on a booking.
UPDATE users u SET marketing_opt_in = true, marketing_opt_in_at = b.at,
    unsubscribe_token = md5(u.id::text || clock_timestamp()::text || random()::text)
FROM (SELECT guest_user_id, min(consent_at) AS at FROM bookings
      WHERE marketing_opt_in AND guest_user_id IS NOT NULL GROUP BY guest_user_id) b
WHERE b.guest_user_id = u.id;

CREATE TABLE campaigns (
    id            uuid        PRIMARY KEY,
    kind          text        NOT NULL CHECK (kind IN ('OFFER', 'FESTIVE', 'ANNOUNCEMENT')),
    subject       text        NOT NULL CHECK (length(subject) BETWEEN 3 AND 150),
    body          text        NOT NULL CHECK (length(body) BETWEEN 10 AND 4000),
    promo_code_id uuid        REFERENCES promo_codes (id) ON DELETE SET NULL,
    status        text        NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED', 'SENDING', 'SENT', 'CANCELLED')),
    send_at       timestamptz NOT NULL DEFAULT now(),
    created_by    uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    sent_at       timestamptz,
    recipients    int         NOT NULL DEFAULT 0
);
-- An offer is announced once, however often it's edited afterwards.
CREATE UNIQUE INDEX campaigns_one_per_offer ON campaigns (promo_code_id) WHERE kind = 'OFFER';
CREATE INDEX campaigns_due_idx ON campaigns (send_at) WHERE status IN ('SCHEDULED', 'SENDING');

CREATE TABLE campaign_recipients (
    campaign_id uuid        NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    queued_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, user_id)
);

ALTER TABLE campaigns ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_recipients ENABLE ROW LEVEL SECURITY;

ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_kind_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_kind_check CHECK (kind IN (
    'OTP', 'WELCOME', 'BOOKING_CONFIRMATION', 'ENQUIRY_REPLY', 'ALERT', 'REFUND_NOTICE', 'STAFF_MESSAGE',
    'STAFF_CODE', 'CHECKIN_REMINDER', 'CHECKOUT_REMINDER', 'PROMOTION'));

-- +goose Down
DELETE FROM outbox_messages WHERE kind = 'PROMOTION';
ALTER TABLE outbox_messages DROP CONSTRAINT outbox_messages_kind_check;
ALTER TABLE outbox_messages ADD CONSTRAINT outbox_messages_kind_check CHECK (kind IN (
    'OTP', 'WELCOME', 'BOOKING_CONFIRMATION', 'ENQUIRY_REPLY', 'ALERT', 'REFUND_NOTICE', 'STAFF_MESSAGE',
    'STAFF_CODE', 'CHECKIN_REMINDER', 'CHECKOUT_REMINDER'));
DROP TABLE IF EXISTS campaign_recipients;
DROP TABLE IF EXISTS campaigns;
ALTER TABLE users DROP COLUMN IF EXISTS unsubscribe_token, DROP COLUMN IF EXISTS marketing_opt_in_at, DROP COLUMN IF EXISTS marketing_opt_in;

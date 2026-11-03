-- +goose Up
-- Offers: a promo can apply by itself (a festive or seasonal offer) instead of waiting for the
-- guest to type its code. Staff manage both kinds from the console (permission promos:write).
ALTER TABLE promo_codes
    ADD COLUMN auto_apply boolean     NOT NULL DEFAULT false,
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

-- Every quote looks for offers that apply today; this keeps that lookup to live offers only.
CREATE INDEX promo_codes_auto_idx ON promo_codes (book_from, book_to) WHERE auto_apply AND is_active;

-- +goose Down
DROP INDEX IF EXISTS promo_codes_auto_idx;
ALTER TABLE promo_codes DROP COLUMN IF EXISTS updated_at, DROP COLUMN IF EXISTS auto_apply;

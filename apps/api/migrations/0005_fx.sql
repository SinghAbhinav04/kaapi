-- +goose Up
-- Exchange rates for showing prices in a guest's own currency. Bookings are always charged
-- in INR; these are only for the "≈ US$102" estimate. A rate is INR per one unit, times
-- 1,000,000, so no floats are stored.
CREATE TABLE fx_rates (
    currency            text        PRIMARY KEY CHECK (currency IN ('USD', 'EUR', 'GBP', 'AUD', 'CAD', 'SGD', 'CHF', 'JPY')),
    inr_per_unit_micros bigint      NOT NULL CHECK (inr_per_unit_micros > 0),
    rate_date           date        NOT NULL,  -- the ECB publication date
    fetched_at          timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE fx_rates ENABLE ROW LEVEL SECURITY;

-- What the guest was looking at when they booked, for analytics. The rate is a snapshot:
-- it never changes what was charged (total_paise stays the only amount).
ALTER TABLE bookings
    ADD COLUMN display_currency       text   CHECK (display_currency IN ('INR', 'USD', 'EUR', 'GBP', 'AUD', 'CAD', 'SGD', 'CHF', 'JPY')),
    ADD COLUMN fx_inr_per_unit_micros bigint CHECK (fx_inr_per_unit_micros > 0),
    ADD COLUMN fx_rate_date           date;

-- +goose Down
ALTER TABLE bookings DROP COLUMN IF EXISTS fx_rate_date, DROP COLUMN IF EXISTS fx_inr_per_unit_micros, DROP COLUMN IF EXISTS display_currency;
DROP TABLE IF EXISTS fx_rates;

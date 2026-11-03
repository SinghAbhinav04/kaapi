-- +goose Up
-- Change data capture, the light way: every write to the tables below sends a topic on the
-- 'data_changes' channel, from whichever process made it, delivered on commit. The API keeps
-- one LISTEN connection; it drops cached analytics and tells open staff consoles to refetch.
-- Statement-level triggers: a bulk rate edit sends one notice, not one per night, and Postgres
-- folds identical notices within a transaction anyway.
--   bookings   a booking made, paid, cancelled, refunded, archived (analytics)
--   inventory  prices, allotments, restrictions, sold counts, holds (rates grid, analytics)
--   offers     promo codes and automatic offers (Offers page, prices)
--   enquiries  enquiries arriving or changing (analytics' enquiry figures)
-- +goose StatementBegin
CREATE FUNCTION notify_data_change() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
BEGIN
    PERFORM pg_notify('data_changes', TG_ARGV[0]);
    RETURN NULL;
END $$;
-- +goose StatementEnd

CREATE TRIGGER bookings_data_change AFTER INSERT OR UPDATE OR DELETE ON bookings
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('bookings');
CREATE TRIGGER night_inventory_data_change AFTER INSERT OR UPDATE OR DELETE ON night_inventory
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER night_rates_data_change AFTER INSERT OR UPDATE OR DELETE ON night_rates
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER holds_data_change AFTER INSERT OR UPDATE OR DELETE ON holds
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER promo_codes_data_change AFTER INSERT OR UPDATE OR DELETE ON promo_codes
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('offers');
CREATE TRIGGER enquiries_data_change AFTER INSERT OR UPDATE OR DELETE ON enquiries
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('enquiries');

-- +goose Down
DROP TRIGGER IF EXISTS enquiries_data_change ON enquiries;
DROP TRIGGER IF EXISTS promo_codes_data_change ON promo_codes;
DROP TRIGGER IF EXISTS holds_data_change ON holds;
DROP TRIGGER IF EXISTS night_rates_data_change ON night_rates;
DROP TRIGGER IF EXISTS night_inventory_data_change ON night_inventory;
DROP TRIGGER IF EXISTS bookings_data_change ON bookings;
DROP FUNCTION IF EXISTS notify_data_change();

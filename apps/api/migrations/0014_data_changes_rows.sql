-- +goose Up
-- Only real changes notify. A statement-level trigger fires even when its statement touched no
-- rows, and the unpaid-booking sweep runs UPDATE bookings / UPDATE holds every minute, usually
-- matching nothing: every console refetched, and the analytics cache emptied, once a minute for
-- no reason. Each trigger now sees the rows the statement changed (a transition table) and
-- stays quiet when there are none. Bulk edits still send one notice per statement.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_data_change() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM changed) THEN
        PERFORM pg_notify('data_changes', TG_ARGV[0]);
    END IF;
    RETURN NULL;
END $$;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS bookings_data_change ON bookings;
CREATE TRIGGER bookings_data_change_ins AFTER INSERT ON bookings REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('bookings');
CREATE TRIGGER bookings_data_change_upd AFTER UPDATE ON bookings REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('bookings');
CREATE TRIGGER bookings_data_change_del AFTER DELETE ON bookings REFERENCING OLD TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('bookings');
DROP TRIGGER IF EXISTS night_inventory_data_change ON night_inventory;
CREATE TRIGGER night_inventory_data_change_ins AFTER INSERT ON night_inventory REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER night_inventory_data_change_upd AFTER UPDATE ON night_inventory REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER night_inventory_data_change_del AFTER DELETE ON night_inventory REFERENCING OLD TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
DROP TRIGGER IF EXISTS night_rates_data_change ON night_rates;
CREATE TRIGGER night_rates_data_change_ins AFTER INSERT ON night_rates REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER night_rates_data_change_upd AFTER UPDATE ON night_rates REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER night_rates_data_change_del AFTER DELETE ON night_rates REFERENCING OLD TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
DROP TRIGGER IF EXISTS holds_data_change ON holds;
CREATE TRIGGER holds_data_change_ins AFTER INSERT ON holds REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER holds_data_change_upd AFTER UPDATE ON holds REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
CREATE TRIGGER holds_data_change_del AFTER DELETE ON holds REFERENCING OLD TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
DROP TRIGGER IF EXISTS promo_codes_data_change ON promo_codes;
CREATE TRIGGER promo_codes_data_change_ins AFTER INSERT ON promo_codes REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('offers');
CREATE TRIGGER promo_codes_data_change_upd AFTER UPDATE ON promo_codes REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('offers');
CREATE TRIGGER promo_codes_data_change_del AFTER DELETE ON promo_codes REFERENCING OLD TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('offers');
DROP TRIGGER IF EXISTS enquiries_data_change ON enquiries;
CREATE TRIGGER enquiries_data_change_ins AFTER INSERT ON enquiries REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('enquiries');
CREATE TRIGGER enquiries_data_change_upd AFTER UPDATE ON enquiries REFERENCING NEW TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('enquiries');
CREATE TRIGGER enquiries_data_change_del AFTER DELETE ON enquiries REFERENCING OLD TABLE AS changed
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('enquiries');

-- +goose Down
DROP TRIGGER IF EXISTS bookings_data_change_ins ON bookings;
DROP TRIGGER IF EXISTS bookings_data_change_upd ON bookings;
DROP TRIGGER IF EXISTS bookings_data_change_del ON bookings;
CREATE TRIGGER bookings_data_change AFTER INSERT OR UPDATE OR DELETE ON bookings
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('bookings');
DROP TRIGGER IF EXISTS night_inventory_data_change_ins ON night_inventory;
DROP TRIGGER IF EXISTS night_inventory_data_change_upd ON night_inventory;
DROP TRIGGER IF EXISTS night_inventory_data_change_del ON night_inventory;
CREATE TRIGGER night_inventory_data_change AFTER INSERT OR UPDATE OR DELETE ON night_inventory
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
DROP TRIGGER IF EXISTS night_rates_data_change_ins ON night_rates;
DROP TRIGGER IF EXISTS night_rates_data_change_upd ON night_rates;
DROP TRIGGER IF EXISTS night_rates_data_change_del ON night_rates;
CREATE TRIGGER night_rates_data_change AFTER INSERT OR UPDATE OR DELETE ON night_rates
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
DROP TRIGGER IF EXISTS holds_data_change_ins ON holds;
DROP TRIGGER IF EXISTS holds_data_change_upd ON holds;
DROP TRIGGER IF EXISTS holds_data_change_del ON holds;
CREATE TRIGGER holds_data_change AFTER INSERT OR UPDATE OR DELETE ON holds
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('inventory');
DROP TRIGGER IF EXISTS promo_codes_data_change_ins ON promo_codes;
DROP TRIGGER IF EXISTS promo_codes_data_change_upd ON promo_codes;
DROP TRIGGER IF EXISTS promo_codes_data_change_del ON promo_codes;
CREATE TRIGGER promo_codes_data_change AFTER INSERT OR UPDATE OR DELETE ON promo_codes
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('offers');
DROP TRIGGER IF EXISTS enquiries_data_change_ins ON enquiries;
DROP TRIGGER IF EXISTS enquiries_data_change_upd ON enquiries;
DROP TRIGGER IF EXISTS enquiries_data_change_del ON enquiries;
CREATE TRIGGER enquiries_data_change AFTER INSERT OR UPDATE OR DELETE ON enquiries
    FOR EACH STATEMENT EXECUTE FUNCTION notify_data_change('enquiries');
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_data_change() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, public AS $$
BEGIN
    PERFORM pg_notify('data_changes', TG_ARGV[0]);
    RETURN NULL;
END $$;
-- +goose StatementEnd

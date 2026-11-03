package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/pii"
	"kaapi/api/internal/pricing"
)

// Options controls a seed run.
type Options struct {
	DemoPassword string   // hashed for every staff account; never stored in the repo
	Reset        bool     // wipe existing data first
	Admin        *Account // the deployment's own administrator, from the environment
}

// Summary is printed after seeding.
type Summary struct {
	RoomTypes, Nights, Rates, Bookings, Confirmed, Enquiries, Users int
}

var ErrNotEmpty = errors.New("seed: the database already has data (run with -reset to replace it)")

const consentVersion = "2026-10"

// Run inserts every fixture in one transaction. Nothing is committed unless every
// invariant check at the end passes.
func Run(ctx context.Context, pool *pgxpool.Pool, fx *Fixtures, opt Options) (Summary, error) {
	var sum Summary
	if opt.DemoPassword == "" {
		return sum, errors.New("seed: SEED_DEMO_PASSWORD is empty")
	}
	staffHash, err := auth.HashPassword(opt.DemoPassword)
	if err != nil {
		return sum, err
	}
	nights, rates, err := ExpandRates(fx.Rules, fx.RoomTypes, fx.Plans)
	if err != nil {
		return sum, err
	}

	err = db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if opt.Reset {
			if _, err := tx.Exec(ctx, `TRUNCATE property_settings, users, room_types, promo_codes, kb_facts,
				enquiries, bookings, holds, payments, payment_events, outbox_messages, notifications, audit_logs,
				otp_codes, sessions, rate_limit_buckets, idempotency_keys CASCADE`); err != nil {
				return err
			}
			// Roles the administrator created go too; built-in roles and their grants stay.
			if _, err := tx.Exec(ctx, `DELETE FROM roles WHERE NOT is_system AND key NOT IN ('GM', 'RESERVATIONS', 'SALES')`); err != nil {
				return err
			}
		} else {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM room_types`).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return ErrNotEmpty
			}
		}

		// property
		slabs, _ := json.Marshal(fx.Property.GSTSlabs)
		lanes, _ := json.Marshal(fx.Property.Lanes)
		if _, err := tx.Exec(ctx, `INSERT INTO property_settings
			(name, timezone, currency, check_in_time, check_out_time, hold_minutes, max_nights, max_rooms_per_booking, child_age_max, gst_slabs, lane_rules)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			fx.Property.Name, fx.Property.Timezone, fx.Property.Currency, fx.Property.CheckInTime, fx.Property.CheckOutTime,
			fx.Property.HoldMinutes, fx.Property.MaxNights, fx.Property.MaxRoomsPerBooking, fx.Property.ChildAgeMax, slabs, lanes); err != nil {
			return fmt.Errorf("property: %w", err)
		}
		for _, k := range fx.Property.KBFacts {
			if _, err := tx.Exec(ctx, `INSERT INTO kb_facts (id, key, category, question, answer) VALUES ($1,$2,$3,$4,$5)`,
				newID(), k.Key, k.Category, k.Question, k.Answer); err != nil {
				return fmt.Errorf("kb fact %s: %w", k.Key, err)
			}
		}

		// users
		userIDs := map[string]uuid.UUID{}
		for _, u := range fx.Users {
			id := newID()
			var hash *string
			if u.Role.IsStaff() {
				hash = &staffHash
			}
			if _, err := tx.Exec(ctx, `INSERT INTO users (id, email, email_hash, name, phone, role, password_hash) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				id, pii.Text(u.Email), pii.EmailIndex(u.Email), pii.Text(u.Name), pii.Null(u.Phone), u.Role, hash); err != nil {
				return fmt.Errorf("user %s: %w", u.Email, err)
			}
			userIDs[strings.ToLower(u.Email)] = id
			sum.Users++
		}
		if opt.Admin != nil {
			if err := ensureAdmin(ctx, tx, *opt.Admin); err != nil {
				return err
			}
			sum.Users++
		}

		// catalogue
		roomIDs := map[string]uuid.UUID{}
		rooms := map[string]RoomType{}
		planIDs := map[string]uuid.UUID{} // "slug/CODE"
		for _, rt := range fx.RoomTypes {
			id := newID()
			roomIDs[rt.Slug], rooms[rt.Slug] = id, rt
			if _, err := tx.Exec(ctx, `INSERT INTO room_types
				(id, slug, name, kind, total_keys, size_sqm, bed, view, base_occupancy, max_adults, max_children,
				 extra_adult_paise, extra_child_paise, summary, description, amenities, sort)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
				id, rt.Slug, rt.Name, rt.Kind, rt.TotalKeys, rt.SizeSqm, rt.Bed, rt.View, rt.BaseOccupancy, rt.MaxAdults,
				rt.MaxChildren, rt.ExtraAdultPaise, rt.ExtraChildPaise, rt.Summary, rt.Description, rt.Amenities, rt.Sort); err != nil {
				return fmt.Errorf("room type %s: %w", rt.Slug, err)
			}
			for i, ph := range rt.Photos {
				if _, err := tx.Exec(ctx, `INSERT INTO room_type_photos (id, room_type_id, url, alt, sort) VALUES ($1,$2,$3,$4,$5)`,
					newID(), id, ph.Path, ph.Alt, i); err != nil {
					return err
				}
			}
			for i, p := range fx.Plans {
				pid := newID()
				planIDs[rt.Slug+"/"+p.Code] = pid
				if _, err := tx.Exec(ctx, `INSERT INTO rate_plans
					(id, room_type_id, code, name, inclusions, refundable, cancellation_policy, min_nights, add_paise_per_night, discount_bps, sort)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
					pid, id, p.Code, p.Name, p.Inclusions, p.Refundable, p.CancellationPolicy, p.MinNights,
					p.Modifier.AddPaisePerNight, p.Modifier.DiscountBps, i); err != nil {
					return fmt.Errorf("plan %s/%s: %w", rt.Slug, p.Code, err)
				}
			}
			sum.RoomTypes++
		}

		// inventory and rates, in bulk
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"night_inventory"},
			[]string{"room_type_id", "date", "allotment", "stop_sell", "stop_sell_reason", "min_stay", "closed_to_arrival", "closed_to_departure"},
			pgx.CopyFromSlice(len(nights), func(i int) ([]any, error) {
				n := nights[i]
				var reason *string
				if n.StopSellReason != "" {
					reason = &n.StopSellReason
				}
				return []any{roomIDs[n.RoomType], n.Date, n.Allotment, n.StopSell, reason, n.MinStay, n.ClosedToArrival, n.ClosedToDeparture}, nil
			})); err != nil {
			return fmt.Errorf("night inventory: %w", err)
		}
		priceOf := make(map[string]int64, len(rates)) // "slug/CODE/2026-11-14"
		for _, r := range rates {
			priceOf[r.RoomType+"/"+r.PlanCode+"/"+r.Date.Format(time.DateOnly)] = r.PricePaise
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"night_rates"}, []string{"rate_plan_id", "date", "price_paise"},
			pgx.CopyFromSlice(len(rates), func(i int) ([]any, error) {
				r := rates[i]
				return []any{planIDs[r.RoomType+"/"+r.PlanCode], r.Date, r.PricePaise}, nil
			})); err != nil {
			return fmt.Errorf("night rates: %w", err)
		}
		sum.Nights, sum.Rates = len(nights), len(rates)

		// promos
		promoIDs := map[string]uuid.UUID{}
		promoRules := map[string]pricing.PromoRule{}
		for _, p := range fx.Promos {
			id := newID()
			var rtIDs []uuid.UUID
			for _, slug := range p.RoomTypes {
				rtIDs = append(rtIDs, roomIDs[slug])
			}
			if _, err := tx.Exec(ctx, `INSERT INTO promo_codes
				(id, code, type, value, description, stay_from, stay_to, book_from, book_to, min_nights, max_uses, room_type_ids, is_active)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
				id, p.Code, p.Type, p.Value, p.Description, p.StayFrom, p.StayTo, p.BookFrom, p.BookTo, p.MinNights, p.MaxUses, rtIDs, p.IsActive); err != nil {
				return fmt.Errorf("promo %s: %w", p.Code, err)
			}
			promoIDs[p.Code] = id
			promoRules[p.Code] = pricing.PromoRule{
				Promo:    pricing.Promo{Code: p.Code, Type: p.Type, Value: p.Value},
				StayFrom: mustDay(p.StayFrom), StayTo: mustDay(p.StayTo), BookFrom: mustDay(p.BookFrom), BookTo: mustDay(p.BookTo),
				MinNights: p.MinNights, MaxUses: p.MaxUses, RoomTypeIDs: rtIDs, Active: p.IsActive,
			}
		}

		// bookings, priced with the real engine
		for _, b := range fx.Bookings {
			if err := insertBooking(ctx, tx, b, rooms[b.RoomType], roomIDs[b.RoomType], planIDs[b.RoomType+"/"+b.RatePlan],
				priceOf, fx.Property.GSTSlabs, promoIDs, promoRules, userIDs); err != nil {
				return fmt.Errorf("booking %s: %w", b.Ref, err)
			}
			sum.Bookings++
			if b.Status == domain.BookingConfirmed {
				sum.Confirmed++
			}
		}

		// enquiries arrive unclassified; the AI worker picks up every PENDING one
		for _, e := range fx.Enquiries {
			if err := insertEnquiry(ctx, tx, e); err != nil {
				return fmt.Errorf("enquiry %s: %w", e.Ref, err)
			}
			sum.Enquiries++
		}

		return checkInvariants(ctx, tx)
	})
	return sum, err
}

func insertBooking(ctx context.Context, tx pgx.Tx, b BookingFx, rt RoomType, roomID, planID uuid.UUID,
	priceOf map[string]int64, slabs []pricing.Slab, promoIDs map[string]uuid.UUID, promoRules map[string]pricing.PromoRule,
	userIDs map[string]uuid.UUID) error {

	in, out := mustDay(b.CheckIn), mustDay(b.CheckOut)
	created, err := time.Parse(time.RFC3339, b.CreatedAt)
	if err != nil {
		return err
	}
	var nights []pricing.Night
	for d := in; d.Before(out); d = d.AddDate(0, 0, 1) {
		p, ok := priceOf[b.RoomType+"/"+b.RatePlan+"/"+d.Format(time.DateOnly)]
		if !ok {
			return fmt.Errorf("no rate on %s", d.Format(time.DateOnly))
		}
		nights = append(nights, pricing.Night{Date: d, PricePaise: p})
	}

	var promo *pricing.Promo
	var promoID *uuid.UUID
	if b.PromoCode != "" {
		rule, ok := promoRules[b.PromoCode]
		if !ok {
			return fmt.Errorf("unknown promo %s", b.PromoCode)
		}
		bookedOn := time.Date(created.Year(), created.Month(), created.Day(), 0, 0, 0, 0, time.UTC)
		if err := pricing.Eligible(rule, domain.Stay{CheckIn: in, CheckOut: out}, bookedOn, roomID); err != nil {
			return fmt.Errorf("promo %s: %w", b.PromoCode, err)
		}
		promo, promoID = &rule.Promo, ptr(promoIDs[b.PromoCode])
	}

	q, err := pricing.Price(pricing.Input{
		Nights: nights, Party: domain.Party{Rooms: b.Rooms, Adults: b.Adults, Children: b.Children}, Promo: promo, Slabs: slabs,
		Room: pricing.Room{BaseOccupancy: rt.BaseOccupancy, MaxAdults: rt.MaxAdults, MaxChildren: rt.MaxChildren, ExtraAdultPaise: rt.ExtraAdultPaise},
	})
	if err != nil {
		return err
	}
	breakdown, _ := json.Marshal(q)

	id := newID()
	var guestUser *uuid.UUID
	if uid, ok := userIDs[strings.ToLower(b.Guest.Email)]; ok {
		guestUser = &uid
	}
	var confirmedAt, cancelledAt *time.Time
	switch b.Status {
	case domain.BookingConfirmed:
		confirmedAt = ptr(created.Add(3 * time.Minute))
	case domain.BookingCancelled, domain.BookingRefundPending, domain.BookingRefunded:
		confirmedAt = ptr(created.Add(3 * time.Minute))
		cancelledAt = ptr(in.AddDate(0, 0, -10))
	}
	var requests *string
	if b.Requests != "" {
		requests = &b.Requests
	}
	if _, err := tx.Exec(ctx, `INSERT INTO bookings
		(id, ref, guest_user_id, guest_name, guest_email, guest_phone, guest_country, adults, children, rooms,
		 room_type_id, rate_plan_id, check_in, check_out, special_requests, status,
		 subtotal_paise, discount_paise, tax_paise, total_paise, price_breakdown, promo_code_id,
		 consent_version, consent_at, source, created_at, updated_at, confirmed_at, cancelled_at, guest_email_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$27,$24,$24,$25,$26,$28)`,
		id, b.Ref, guestUser, pii.Text(b.Guest.Name), pii.Text(b.Guest.Email), pii.Text(b.Guest.Phone), pii.Text(b.Guest.Country),
		b.Adults, b.Children, b.Rooms,
		roomID, planID, in, out, requests, b.Status,
		q.SubtotalPaise, q.DiscountPaise, q.TaxPaise, q.TotalPaise, breakdown, promoID,
		consentVersion, created, confirmedAt, cancelledAt, domain.BookingSourceDirectWeb, pii.EmailIndex(b.Guest.Email)); err != nil {
		return err
	}

	// Only confirmed bookings hold inventory. The CHECK (sold <= allotment) would stop an oversold fixture here.
	if b.Status.HoldsInventory() {
		for d := in; d.Before(out); d = d.AddDate(0, 0, 1) {
			if _, err := tx.Exec(ctx, `INSERT INTO booking_nights (booking_id, date, room_type_id, rooms) VALUES ($1,$2,$3,$4)`,
				id, d, roomID, b.Rooms); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `UPDATE night_inventory SET sold = sold + $3, updated_at = now()
				WHERE room_type_id = $1 AND date = $2 AND NOT stop_sell`, roomID, d, b.Rooms)
			if err != nil {
				return fmt.Errorf("%s: %w", d.Format(time.DateOnly), err)
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("%s is closed for sale", d.Format(time.DateOnly))
			}
		}
		if promoID != nil {
			if _, err := tx.Exec(ctx, `UPDATE promo_codes SET used_count = used_count + 1 WHERE id = $1`, *promoID); err != nil {
				return err
			}
		}
	}

	payStatus, reason, providerPay := paymentFor[b.Status], (*string)(nil), ptr("pay_mock_"+strings.TrimPrefix(b.Ref, "KS-"))
	if payStatus == "" {
		return nil // EXPIRED: the guest never paid
	}
	if b.Payment != nil {
		payStatus, providerPay = b.Payment.Status, &b.Payment.ProviderPaymentID
		if b.Payment.FailureReason != "" {
			reason = &b.Payment.FailureReason
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO payments
		(id, booking_id, provider, provider_order_id, provider_payment_id, amount_paise, status, idempotency_key, failure_reason, created_at, updated_at)
		VALUES ($1,$2,$10,$3,$4,$5,$6,$7,$8,$9,$9)`,
		newID(), id, "order_"+b.Ref, providerPay, q.TotalPaise, payStatus, "seed-"+b.Ref, reason, created.Add(2*time.Minute), domain.PaymentProviderMock)
	return err
}

func insertEnquiry(ctx context.Context, tx pgx.Tx, e EnquiryFx) error {
	received, err := time.Parse(time.RFC3339, e.ReceivedAt)
	if err != nil {
		return err
	}
	var firstResponse *time.Time
	if e.SeedState.FirstResponseMinutes != nil {
		firstResponse = ptr(received.Add(time.Duration(*e.SeedState.FirstResponseMinutes) * time.Minute))
	}
	_, err = tx.Exec(ctx, `INSERT INTO enquiries
		(id, ref, channel, sender_name, sender_email, sender_phone, subject, raw_text, received_at, dedupe_hash, status, first_response_at, created_at, updated_at, sender_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$9,$9,$13)`,
		newID(), e.Ref, e.Channel, pii.Null(nilIfEmpty(e.Sender.Name)), pii.Null(nilIfEmpty(e.Sender.Email)), pii.Null(nilIfEmpty(e.Sender.Phone)),
		nilIfEmpty(e.Subject), e.Text, received, enquiry.DedupeHash(e.Text), e.SeedState.Status, firstResponse,
		enquiry.SenderIndex(e.Sender.Email, e.Sender.Phone))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO enquiry_events (id, enquiry_id, type, actor, created_at)
		SELECT $1, id, $3, $4, received_at FROM enquiries WHERE ref = $2`, newID(), e.Ref, domain.EventReceived, domain.ActorSystem)
	return err
}

// checkInvariants refuses to commit demo data that the running app would consider corrupt.
func checkInvariants(ctx context.Context, tx pgx.Tx) error {
	var mismatched int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM night_inventory ni
		LEFT JOIN (SELECT room_type_id, date, sum(rooms) AS rooms FROM booking_nights GROUP BY 1, 2) bn
		  ON bn.room_type_id = ni.room_type_id AND bn.date = ni.date
		WHERE ni.sold <> coalesce(bn.rooms, 0)`).Scan(&mismatched); err != nil {
		return err
	}
	if mismatched > 0 {
		return fmt.Errorf("invariant: %d nights where sold doesn't match booked nights", mismatched)
	}
	return nil
}

// paymentFor is the payment state that goes with each seeded booking state.
// EXPIRED and PENDING_PAYMENT have no payment: the guest never paid.
var paymentFor = map[domain.BookingStatus]domain.PaymentStatus{
	domain.BookingConfirmed:     domain.PaymentCaptured,
	domain.BookingPaymentFailed: domain.PaymentFailed,
	domain.BookingRefundPending: domain.PaymentRefundPending,
	domain.BookingRefunded:      domain.PaymentRefunded,
	domain.BookingCancelled:     domain.PaymentRefunded,
}

func newID() uuid.UUID { return idgen.New() }

func ptr[T any](v T) *T { return &v }

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func mustDay(s string) time.Time {
	d, err := parseDay(s)
	if err != nil {
		panic(err)
	}
	return d
}

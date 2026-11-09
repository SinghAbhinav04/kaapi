package inventory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
)

// CreateHold reserves rooms for the property's hold window (10 minutes in the seed data).
//
// Inside one transaction it:
//  1. locks every night of the stay (SELECT … FOR UPDATE, ordered by date),
//  2. checks stop-sell, minimum stay and closed-to-arrival/departure,
//  3. counts live holds (ACTIVE and not yet expired) on each night,
//  4. refuses if sold + held + requested > allotment on any night, naming those nights,
//  5. prices the stay and inserts the hold with a frozen quote.
//
// Because step 1 serialises every writer on the same nights, concurrent requests for the
// last room queue up behind each other and exactly one of them wins.
func (s *Service) CreateHold(ctx context.Context, req HoldRequest) (Hold, error) {
	st, err := loadSettings(ctx, s.pool)
	if err != nil {
		return Hold{}, err
	}
	if err := validateStay(req.Stay, req.Party, st, s.today(st)); err != nil {
		return Hold{}, err
	}
	token, tokenHash, err := newToken()
	if err != nil {
		return Hold{}, err
	}

	var hold Hold
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		nights, err := lockNights(ctx, tx, req.RoomTypeID, req.Stay)
		if err != nil {
			return err
		}
		if err := checkRestrictions(ctx, tx, req.RoomTypeID, req.CheckOut, nights); err != nil {
			return err
		}
		if err := countHolds(ctx, tx, req.RoomTypeID, nights, uuid.Nil); err != nil {
			return err
		}
		if err := checkCapacity(nights, req.Rooms); err != nil {
			return err
		}

		quote, promoID, err := quoteStay(ctx, tx, req, st, s.today(st))
		if err != nil {
			return err
		}
		quoteJSON, err := json.Marshal(quote)
		if err != nil {
			return err
		}

		hold = Hold{ID: idgen.New(), Token: token, RoomTypeID: req.RoomTypeID, RatePlanID: req.RatePlanID,
			Stay: req.Stay, Rooms: req.Rooms, Quote: quote}
		hold.ExpiresAt, err = store{}.insertHold(ctx, tx, hold, req, tokenHash, quoteJSON, promoID, st.holdMinutes)
		return err
	})
	return hold, err
}

// VerifyToken checks a guest's hold token in constant time.
func (s *Service) VerifyToken(ctx context.Context, holdID uuid.UUID, token string) error {
	stored, err := store{}.holdTokenHash(ctx, s.pool, holdID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrHoldNotFound
	}
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(sum[:], stored) != 1 {
		return ErrHoldNotFound // don't reveal that the hold exists
	}
	return nil
}

// Release gives the rooms back early, for example when the guest leaves checkout.
// A hold with a booking waiting for its payment is kept: the payment may already be on its
// way, and releasing the room would send it to a refund. It runs out on its own if unpaid.
func (s *Service) Release(ctx context.Context, holdID uuid.UUID) error {
	paying, err := store{}.awaitingPayment(ctx, s.pool, holdID)
	if err != nil || paying {
		return err
	}
	released, err := store{}.release(ctx, s.pool, holdID)
	if err != nil {
		return err
	}
	if !released {
		return ErrHoldNotActive
	}
	return nil
}

// Confirm turns a hold into sold inventory for a booking, inside the caller's transaction
// (the payment webhook also updates the booking and payment in the same transaction).
//
// If the hold expired while the guest was paying, Confirm re-checks capacity: when a room
// is still free the booking goes through; otherwise it returns a SoldOutError and the
// caller marks the payment for refund.
func Confirm(ctx context.Context, tx pgx.Tx, holdID, bookingID uuid.UUID) error {
	st := store{}
	h, err := st.lockHold(ctx, tx, holdID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrHoldNotFound
	}
	if err != nil {
		return err
	}
	if h.status != domain.HoldActive {
		return ErrHoldNotActive
	}
	roomTypeID, rooms := h.roomTypeID, h.rooms

	nights, err := lockNights(ctx, tx, roomTypeID, h.stay)
	if err != nil {
		return err
	}
	if h.expired {
		// Late payment: only go ahead if the rooms are still free without our hold.
		if err := countHolds(ctx, tx, roomTypeID, nights, holdID); err != nil {
			return err
		}
		if err := checkCapacity(nights, rooms); err != nil {
			if uerr := st.setHoldStatus(ctx, tx, holdID, domain.HoldExpired); uerr != nil {
				return uerr
			}
			return err
		}
	}

	for _, n := range nights {
		if err := st.sellNight(ctx, tx, roomTypeID, bookingID, n.date, rooms); err != nil {
			return err
		}
	}
	return st.setHoldStatus(ctx, tx, holdID, domain.HoldConverted)
}

// lockNights locks the stay's nights in date order and returns them.
func lockNights(ctx context.Context, tx pgx.Tx, roomTypeID uuid.UUID, stay domain.Stay) ([]night, error) {
	nights, err := store{}.lockNights(ctx, tx, roomTypeID, stay.CheckIn, stay.CheckOut)
	if err != nil {
		return nil, err
	}
	if len(nights) != stay.Nights() {
		return nil, ErrNotForSale
	}
	return nights, nil
}

func checkRestrictions(ctx context.Context, tx pgx.Tx, roomTypeID uuid.UUID, checkOut time.Time, nights []night) error {
	first := nights[0]
	for _, n := range nights {
		if n.stopSell {
			reason := ""
			if n.stopSellReason != nil {
				reason = *n.stopSellReason
			}
			return &RestrictionError{Err: ErrStopSell, Date: n.date, Reason: reason}
		}
	}
	if first.closedToArrival {
		return &RestrictionError{Err: ErrClosedToArrival, Date: first.date}
	}
	if len(nights) < first.minStay {
		return &RestrictionError{Err: ErrMinStay, Date: first.date, MinStay: first.minStay}
	}
	ctd, err := store{}.closedToDeparture(ctx, tx, roomTypeID, checkOut)
	if err != nil {
		return err
	}
	if ctd {
		return &RestrictionError{Err: ErrClosedToDeparture, Date: checkOut}
	}
	return nil
}

// countHolds fills in the rooms held on each night by live holds, optionally ignoring one hold.
func countHolds(ctx context.Context, tx pgx.Tx, roomTypeID uuid.UUID, nights []night, except uuid.UUID) error {
	held, err := store{}.heldPerNight(ctx, tx, roomTypeID, nights[0].date, nights[len(nights)-1].date, except)
	if err != nil {
		return err
	}
	for i := range min(len(held), len(nights)) {
		nights[i].held = held[i]
	}
	return nil
}

func checkCapacity(nights []night, rooms int) error {
	var full []time.Time
	for _, n := range nights {
		if n.sold+n.held+rooms > n.allotment {
			full = append(full, n.date)
		}
	}
	if len(full) > 0 {
		return &SoldOutError{Nights: full}
	}
	return nil
}

func validateStay(stay domain.Stay, party domain.Party, st settings, today time.Time) error {
	n := stay.Nights()
	switch {
	case n < 1:
		return invalidStay("Check-out must be after check-in.")
	case n > st.maxNights:
		return invalidStay(fmt.Sprintf("You can book up to %d nights online. For longer stays, send us an enquiry.", st.maxNights))
	case stay.CheckIn.Before(today):
		return invalidStay("Check-in can't be in the past.")
	case party.Rooms < 1 || party.Rooms > st.maxRooms:
		return invalidStay(fmt.Sprintf("You can book between 1 and %d rooms online.", st.maxRooms))
	case party.Adults < 1 || party.Children < 0:
		return invalidStay("At least one adult needs to be on the booking.")
	}
	return nil
}

func newToken() (string, []byte, error) {
	b := make([]byte, holdTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

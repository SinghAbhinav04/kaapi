package admin

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/db"
	"kaapi/api/internal/platform/idgen"
)

// Offers are promo codes and automatic (festive, seasonal) discounts, managed by staff with
// promos:write. Quotes apply the guest's best offer (inventory.quoteStay); the price itself
// still comes from the rate table, only reduced.

var offerCodeRe = regexp.MustCompile(`^[A-Z0-9]{3,24}$`)

// Offers lists every offer, live ones first, newest first.
func (s *Service) Offers(ctx context.Context) ([]Offer, error) {
	return s.store.offers(ctx, s.pool)
}

// CreateOffer adds an offer.
func (s *Service) CreateOffer(ctx context.Context, actor uuid.UUID, in OfferInput) (Offer, error) {
	if err := validateOffer(&in, 0); err != nil {
		return Offer{}, err
	}
	var out Offer
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.checkRooms(ctx, tx, in.RoomTypeIDs); err != nil {
			return err
		}
		id := idgen.New()
		if err := s.store.insertOffer(ctx, tx, id, in); err != nil {
			return offerWriteError(err)
		}
		var err error
		if out, err = s.store.offer(ctx, tx, id); err != nil {
			return err
		}
		return s.store.auditEntry(ctx, tx, actor, auditActionOfferCreate, auditEntityOffer, id.String(), offerAudit(out))
	})
	return out, err
}

// UpdateOffer replaces an offer's settings. Its use count is kept, so the cap can't be set
// below what has already been used.
func (s *Service) UpdateOffer(ctx context.Context, actor, id uuid.UUID, in OfferInput) (Offer, error) {
	var out Offer
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := s.store.offer(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := validateOffer(&in, cur.UsedCount); err != nil {
			return err
		}
		if err := s.store.checkRooms(ctx, tx, in.RoomTypeIDs); err != nil {
			return err
		}
		if err := s.store.updateOffer(ctx, tx, id, in); err != nil {
			return offerWriteError(err)
		}
		if out, err = s.store.offer(ctx, tx, id); err != nil {
			return err
		}
		return s.store.auditEntry(ctx, tx, actor, auditActionOfferUpdate, auditEntityOffer, id.String(), offerAudit(out))
	})
	return out, err
}

func validateOffer(in *OfferInput, used int) error {
	f := map[string]string{}
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Description = strings.Join(strings.Fields(in.Description), " ")
	if !offerCodeRe.MatchString(in.Code) {
		f["code"] = "must be 3 to 24 capital letters or digits"
	}
	if n := len([]rune(in.Description)); n < offerDescriptionMin || n > offerDescriptionMax {
		f["description"] = fmt.Sprintf("must be %d to %d characters", offerDescriptionMin, offerDescriptionMax)
	}
	switch in.Type {
	case domain.PromoPercent:
		if in.Value < 1 || in.Value > maxPercentBps {
			f["value"] = "must be between 0.01% and 50%"
		}
	case domain.PromoFlat:
		if in.Value < 1 || in.Value > maxFlatPaise {
			f["value"] = "must be between ₹0.01 and ₹1,00,000"
		}
	default:
		f["type"] = "must be PERCENT or FLAT"
	}
	if in.StayFrom.IsZero() || in.StayTo.Before(in.StayFrom) {
		f["stay_to"] = "must be on or after the first stay date"
	}
	if in.BookFrom.IsZero() || in.BookTo.Before(in.BookFrom) {
		f["book_to"] = "must be on or after the first booking date"
	}
	if in.MinNights < 1 || in.MinNights > maxOfferMinNights {
		f["min_nights"] = fmt.Sprintf("must be 1 to %d", maxOfferMinNights)
	}
	if in.MaxUses != nil && (*in.MaxUses < 1 || *in.MaxUses < used) {
		f["max_uses"] = fmt.Sprintf("must be at least %d (already used)", max(used, 1))
	}
	if len(f) > 0 {
		return apperr.Validation(f)
	}
	return nil
}

// offerWriteError turns a duplicate code into a message for people.
func offerWriteError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == pgUniqueViolation {
		return ErrOfferCodeTaken
	}
	return err
}

func offerAudit(o Offer) map[string]any {
	return map[string]any{"code": o.Code, "type": o.Type, "value": o.Value, "auto_apply": o.AutoApply, "active": o.Active,
		"stay":       []string{o.StayFrom.Format(time.DateOnly), o.StayTo.Format(time.DateOnly)},
		"book":       []string{o.BookFrom.Format(time.DateOnly), o.BookTo.Format(time.DateOnly)},
		"min_nights": o.MinNights, "max_uses": o.MaxUses, "rooms": o.RoomTypeIDs}
}

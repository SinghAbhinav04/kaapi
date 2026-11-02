// Package pricing turns nightly rates into a guest-facing quote: extra guests, promo
// discount and GST, night by night. It is pure (no I/O) so every rule is unit-tested.
//
// All money is int64 paise.
package pricing

import (
	"errors"
	"fmt"
	"time"
)

// Slab is one GST band. MaxTariffPaise is the per-room, per-night ceiling for the band;
// nil means "everything above the previous band".
type Slab struct {
	MaxTariffPaise *int64 `json:"max_tariff_paise_per_night"`
	RateBps        int64  `json:"rate_bps"`
}

// Night is the plan price for one room on one night, before extra guests, discount and GST.
type Night struct {
	Date       time.Time
	PricePaise int64
}

// Room holds the occupancy rules of a room type.
type Room struct {
	BaseOccupancy   int
	MaxAdults       int
	MaxChildren     int
	ExtraAdultPaise int64
}

type PromoType string

const (
	PromoPercent PromoType = "PERCENT"
	PromoFlat    PromoType = "FLAT"
)

// Promo is a discount that has already passed eligibility checks (see Eligible).
type Promo struct {
	Code  string
	Type  PromoType
	Value int64 // basis points for PERCENT, paise for FLAT
}

type Input struct {
	Nights   []Night
	Rooms    int
	Adults   int
	Children int
	Room     Room
	Promo    *Promo
	Slabs    []Slab
}

// Line is one night of the quote, for all rooms together.
type Line struct {
	Date          time.Time `json:"date"`
	RoomPaise     int64     `json:"room_paise"`     // plan price × rooms + extra adults
	DiscountPaise int64     `json:"discount_paise"` // promo share for this night
	TaxablePaise  int64     `json:"taxable_paise"`
	GSTRateBps    int64     `json:"gst_rate_bps"`
	GSTPaise      int64     `json:"gst_paise"`
	TotalPaise    int64     `json:"total_paise"`
}

type Quote struct {
	Lines         []Line `json:"lines"`
	SubtotalPaise int64  `json:"subtotal_paise"`
	DiscountPaise int64  `json:"discount_paise"`
	TaxPaise      int64  `json:"tax_paise"`
	TotalPaise    int64  `json:"total_paise"`
	PromoCode     string `json:"promo_code,omitempty"`
}

var (
	ErrNoNights     = errors.New("pricing: at least one night is required")
	ErrNoRooms      = errors.New("pricing: at least one room is required")
	ErrTooManyGuest = errors.New("pricing: too many guests for the rooms chosen")
	ErrBadSlabs     = errors.New("pricing: GST slabs must end with an open band")
)

// Price builds the quote.
//
// Rules (each is a labelled assumption in the TDD):
//   - Adults above BaseOccupancy × rooms pay ExtraAdultPaise per night; children stay free.
//   - A promo is applied before tax. PERCENT is taken night by night; FLAT is spread across
//     nights in proportion to their price, with any rounding remainder on the last night.
//   - The GST band is chosen per night on what one room actually costs after the discount
//     (transaction value), so a promo can move a night into a lower band.
func Price(in Input) (Quote, error) {
	if len(in.Nights) == 0 {
		return Quote{}, ErrNoNights
	}
	if in.Rooms < 1 {
		return Quote{}, ErrNoRooms
	}
	if in.Adults < 1 || in.Adults > in.Room.MaxAdults*in.Rooms || in.Children < 0 || in.Children > in.Room.MaxChildren*in.Rooms {
		return Quote{}, ErrTooManyGuest
	}
	if len(in.Slabs) == 0 || in.Slabs[len(in.Slabs)-1].MaxTariffPaise != nil {
		return Quote{}, ErrBadSlabs
	}

	extraAdults := in.Adults - in.Room.BaseOccupancy*in.Rooms
	if extraAdults < 0 {
		extraAdults = 0
	}

	q := Quote{Lines: make([]Line, len(in.Nights))}
	for i, n := range in.Nights {
		if n.PricePaise <= 0 {
			return Quote{}, fmt.Errorf("pricing: no price for %s", n.Date.Format(time.DateOnly))
		}
		room := n.PricePaise*int64(in.Rooms) + int64(extraAdults)*in.Room.ExtraAdultPaise
		q.Lines[i] = Line{Date: n.Date, RoomPaise: room}
		q.SubtotalPaise += room
	}

	if in.Promo != nil {
		applyPromo(q.Lines, q.SubtotalPaise, *in.Promo)
		q.PromoCode = in.Promo.Code
	}

	for i := range q.Lines {
		l := &q.Lines[i]
		l.TaxablePaise = l.RoomPaise - l.DiscountPaise
		l.GSTRateBps = rateFor(in.Slabs, divRound(l.TaxablePaise, int64(in.Rooms)))
		l.GSTPaise = divRound(l.TaxablePaise*l.GSTRateBps, 10_000)
		l.TotalPaise = l.TaxablePaise + l.GSTPaise
		q.DiscountPaise += l.DiscountPaise
		q.TaxPaise += l.GSTPaise
	}
	q.TotalPaise = q.SubtotalPaise - q.DiscountPaise + q.TaxPaise
	return q, nil
}

func applyPromo(lines []Line, subtotal int64, p Promo) {
	switch p.Type {
	case PromoPercent:
		for i := range lines {
			lines[i].DiscountPaise = divRound(lines[i].RoomPaise*p.Value, 10_000)
		}
	case PromoFlat:
		total := min(p.Value, subtotal)
		var given int64
		for i := range lines {
			if i == len(lines)-1 {
				lines[i].DiscountPaise = total - given
				break
			}
			share := divRound(total*lines[i].RoomPaise, subtotal)
			lines[i].DiscountPaise = share
			given += share
		}
	}
}

func rateFor(slabs []Slab, tariffPerRoom int64) int64 {
	for _, s := range slabs {
		if s.MaxTariffPaise == nil || tariffPerRoom <= *s.MaxTariffPaise {
			return s.RateBps
		}
	}
	return slabs[len(slabs)-1].RateBps
}

// divRound divides and rounds half away from zero (all inputs here are non-negative).
func divRound(a, b int64) int64 {
	return (a + b/2) / b
}

// DerivePlanPrice turns a room-only price into another plan's price:
// (room only + add-on) less discount, rounded to the nearest roundTo paise.
func DerivePlanPrice(roomOnlyPaise, addPaise int64, discountBps int64, roundTo int64) int64 {
	p := divRound((roomOnlyPaise+addPaise)*(10_000-discountBps), 10_000)
	return RoundTo(p, roundTo)
}

// RoundTo rounds to the nearest multiple of unit, halves up.
func RoundTo(v, unit int64) int64 {
	if unit <= 1 {
		return v
	}
	return divRound(v, unit) * unit
}

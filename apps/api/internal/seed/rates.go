package seed

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"kaapi/api/internal/pricing"
)

// NightRow is one room type's inventory for one night.
type NightRow struct {
	RoomType          string
	Date              time.Time
	Allotment         int
	StopSell          bool
	StopSellReason    string
	MinStay           int
	ClosedToArrival   bool
	ClosedToDeparture bool
}

// RateRow is one plan's price for one night.
type RateRow struct {
	RoomType   string
	PlanCode   string
	Date       time.Time
	PricePaise int64
}

// ExpandRates turns the compact rules into one row per room type per night, and one price
// per plan per night: base × (1 + season uplift) × (1 + weekend uplift), rounded to ₹100,
// then each plan derived from the room-only price.
func ExpandRates(rules RateRules, roomTypes []RoomType, plans []Plan) ([]NightRow, []RateRow, error) {
	from, err := parseDay(rules.Window.From)
	if err != nil {
		return nil, nil, err
	}
	to, err := parseDay(rules.Window.To)
	if err != nil {
		return nil, nil, err
	}

	var nights []NightRow
	var rates []RateRow
	for _, rt := range roomTypes {
		base, ok := rules.BaseRatesPaise[rt.Slug]
		if !ok {
			return nil, nil, fmt.Errorf("no base rate for %s", rt.Slug)
		}
		allot, ok := rules.WebsiteAllotment[rt.Slug]
		if !ok {
			return nil, nil, fmt.Errorf("no website allotment for %s", rt.Slug)
		}
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			row := NightRow{RoomType: rt.Slug, Date: d, Allotment: allot, MinStay: 1}
			seasonBps := int64(0)
			for _, s := range rules.Seasons {
				sf, _ := parseDay(s.From)
				st, _ := parseDay(s.To)
				if !d.Before(sf) && !d.After(st) {
					seasonBps = s.UpliftBps
					if s.MinStay > row.MinStay {
						row.MinStay = s.MinStay
					}
					row.ClosedToArrival = slices.Contains(s.ClosedToArrival, d.Format(time.DateOnly))
					break
				}
			}
			for _, o := range rules.AllotmentOverrides {
				of, _ := parseDay(o.From)
				ot, _ := parseDay(o.To)
				if (o.RoomType == "*" || o.RoomType == rt.Slug) && !d.Before(of) && !d.After(ot) {
					if o.StopSell {
						row.StopSell, row.StopSellReason = true, o.Reason
					}
					if o.Allotment != nil {
						row.Allotment = *o.Allotment
					}
				}
			}
			if row.Allotment > rt.TotalKeys {
				return nil, nil, fmt.Errorf("%s on %s: allotment %d is more than %d keys", rt.Slug, d.Format(time.DateOnly), row.Allotment, rt.TotalKeys)
			}
			nights = append(nights, row)

			weekendBps := int64(0)
			if slices.Contains(rules.WeekendNights, strings.ToUpper(d.Weekday().String()[:3])) {
				weekendBps = rules.WeekendUpliftBps
			}
			roomOnly := pricing.RoundTo(base*(10_000+seasonBps)*(10_000+weekendBps)/100_000_000, rules.RoundingPaise)
			for _, p := range plans {
				price := roomOnly
				if p.Modifier.AddPaisePerNight != 0 || p.Modifier.DiscountBps != 0 {
					price = pricing.DerivePlanPrice(roomOnly, p.Modifier.AddPaisePerNight, p.Modifier.DiscountBps, rules.RoundingPaise)
				}
				rates = append(rates, RateRow{RoomType: rt.Slug, PlanCode: p.Code, Date: d, PricePaise: price})
			}
		}
	}
	return nights, rates, nil
}

func parseDay(s string) (time.Time, error) {
	return time.Parse(time.DateOnly, s)
}

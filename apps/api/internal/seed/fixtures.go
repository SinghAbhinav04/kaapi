// Package seed loads the JSON fixtures in seed/data into the database.
// Fixtures hold inputs only; everything derived (nightly rates, sold counts, booking totals)
// is computed here with the real pricing code, so demo data can never disagree with the app.
package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SinghAbhinav04/kaapi-estate/apps/api/internal/pricing"
)

type Fixtures struct {
	Property  Property
	RoomTypes []RoomType
	Plans     []Plan
	Rules     RateRules
	Promos    []PromoFx
	Users     []UserFx
	Enquiries []EnquiryFx
	Bookings  []BookingFx
}

type Property struct {
	Name               string            `json:"name"`
	Timezone           string            `json:"timezone"`
	Currency           string            `json:"currency"`
	CheckInTime        string            `json:"check_in_time"`
	CheckOutTime       string            `json:"check_out_time"`
	HoldMinutes        int               `json:"hold_minutes"`
	MaxNights          int               `json:"max_nights"`
	MaxRoomsPerBooking int               `json:"max_rooms_per_booking"`
	ChildAgeMax        int               `json:"child_age_max"`
	GSTSlabs           []pricing.Slab    `json:"gst_slabs"`
	Lanes              map[string]string `json:"lanes"`
	KBFacts            []struct {
		Key      string `json:"key"`
		Category string `json:"category"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	} `json:"kb_facts"`
}

type RoomType struct {
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Kind            string   `json:"kind"`
	TotalKeys       int      `json:"total_keys"`
	SizeSqm         int      `json:"size_sqm"`
	Bed             string   `json:"bed"`
	View            string   `json:"view"`
	BaseOccupancy   int      `json:"base_occupancy"`
	MaxAdults       int      `json:"max_adults"`
	MaxChildren     int      `json:"max_children"`
	ExtraAdultPaise int64    `json:"extra_adult_paise"`
	ExtraChildPaise int64    `json:"extra_child_paise"`
	Sort            int      `json:"sort"`
	Summary         string   `json:"summary"`
	Description     string   `json:"description"`
	Amenities       []string `json:"amenities"`
	Photos          []struct {
		Path string `json:"path"`
		Alt  string `json:"alt"`
	} `json:"photos"`
}

type Plan struct {
	Code               string   `json:"code"`
	Name               string   `json:"name"`
	Inclusions         []string `json:"inclusions"`
	Refundable         bool     `json:"refundable"`
	CancellationPolicy string   `json:"cancellation_policy"`
	MinNights          int      `json:"min_nights"`
	Modifier           struct {
		AddPaisePerNight int64 `json:"add_paise_per_night"`
		DiscountBps      int64 `json:"discount_bps"`
	} `json:"modifier"`
}

type RateRules struct {
	Window struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"window"`
	RoundingPaise    int64            `json:"rounding_paise"`
	WeekendNights    []string         `json:"weekend_nights"`
	WeekendUpliftBps int64            `json:"weekend_uplift_bps"`
	BaseRatesPaise   map[string]int64 `json:"base_rates_paise"`
	WebsiteAllotment map[string]int   `json:"website_allotment"`
	Seasons          []struct {
		Name            string   `json:"name"`
		From            string   `json:"from"`
		To              string   `json:"to"`
		UpliftBps       int64    `json:"uplift_bps"`
		MinStay         int      `json:"min_stay"`
		ClosedToArrival []string `json:"closed_to_arrival"`
	} `json:"seasons"`
	AllotmentOverrides []struct {
		RoomType  string `json:"room_type"`
		From      string `json:"from"`
		To        string `json:"to"`
		StopSell  bool   `json:"stop_sell"`
		Allotment *int   `json:"allotment"`
		Reason    string `json:"reason"`
	} `json:"allotment_overrides"`
}

type PromoFx struct {
	Code        string   `json:"code"`
	Type        string   `json:"type"`
	Value       int64    `json:"value"`
	Description string   `json:"description"`
	StayFrom    string   `json:"stay_from"`
	StayTo      string   `json:"stay_to"`
	BookFrom    string   `json:"book_from"`
	BookTo      string   `json:"book_to"`
	MinNights   int      `json:"min_nights"`
	MaxUses     *int     `json:"max_uses"`
	RoomTypes   []string `json:"room_types"`
	IsActive    bool     `json:"is_active"`
}

type UserFx struct {
	Email string  `json:"email"`
	Name  string  `json:"name"`
	Role  string  `json:"role"`
	Phone *string `json:"phone"`
}

type EnquiryFx struct {
	Ref        string `json:"ref"`
	Channel    string `json:"channel"`
	ReceivedAt string `json:"received_at"`
	Sender     struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	} `json:"sender"`
	Subject   string `json:"subject"`
	Text      string `json:"text"`
	SeedState struct {
		Status               string `json:"status"`
		FirstResponseMinutes *int   `json:"first_response_minutes"`
	} `json:"seed_state"`
	Expected map[string]any `json:"expected"`
	Tags     []string       `json:"tags"`
}

type BookingFx struct {
	Ref       string `json:"ref"`
	RoomType  string `json:"room_type"`
	RatePlan  string `json:"rate_plan"`
	CheckIn   string `json:"check_in"`
	CheckOut  string `json:"check_out"`
	Rooms     int    `json:"rooms"`
	Adults    int    `json:"adults"`
	Children  int    `json:"children"`
	Status    string `json:"status"`
	PromoCode string `json:"promo_code"`
	CreatedAt string `json:"created_at"`
	Requests  string `json:"special_requests"`
	Guest     struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Country string `json:"country"`
	} `json:"guest"`
	Payment *struct {
		ProviderPaymentID string `json:"provider_payment_id"`
		Status            string `json:"status"`
		FailureReason     string `json:"failure_reason"`
	} `json:"payment"`
}

// Load reads every fixture file from dir.
func Load(dir string) (*Fixtures, error) {
	f := &Fixtures{}
	files := []struct {
		name string
		into any
	}{
		{"property.json", &f.Property},
		{"room_types.json", &struct {
			RoomTypes *[]RoomType `json:"room_types"`
		}{&f.RoomTypes}},
		{"rate_plans.json", &struct {
			Plans *[]Plan `json:"plans"`
		}{&f.Plans}},
		{"rates_rules.json", &f.Rules},
		{"promos.json", &struct {
			Promos *[]PromoFx `json:"promos"`
		}{&f.Promos}},
		{"users.json", &struct {
			Users *[]UserFx `json:"users"`
		}{&f.Users}},
		{"enquiries.json", &struct {
			Enquiries *[]EnquiryFx `json:"enquiries"`
		}{&f.Enquiries}},
		{"bookings.json", &struct {
			Bookings *[]BookingFx `json:"bookings"`
		}{&f.Bookings}},
	}
	for _, file := range files {
		b, err := os.ReadFile(filepath.Join(dir, file.name))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, file.into); err != nil {
			return nil, fmt.Errorf("%s: %w", file.name, err)
		}
	}
	return f, nil
}

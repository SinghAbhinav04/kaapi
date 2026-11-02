// Package seed loads the JSON fixtures in seed/data into the database.
// Fixtures hold inputs only; everything derived (nightly rates, sold counts, booking totals)
// is computed here with the real pricing code, so demo data can never disagree with the app.
package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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

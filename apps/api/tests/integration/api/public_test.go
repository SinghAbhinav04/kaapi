package api_test

import (
	"net/http"
	"testing"

	"kaapi/api/internal/platform/apperr"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

func TestCatalog(t *testing.T) {
	s := apitest.New(t)

	var rooms api.RoomsResponse
	res := s.Do(http.MethodGet, "/v1/rooms", nil, nil)
	res.JSON(t, &rooms)
	if res.Status != http.StatusOK || len(rooms.Rooms) != 5 {
		t.Fatalf("%d, %d rooms", res.Status, len(rooms.Rooms))
	}

	var one api.RoomResponse
	s.Do(http.MethodGet, "/v1/rooms/coffee-villa", nil, nil).JSON(t, &one)
	if one.Room.Name != "Coffee Villa" || len(one.Room.Plans) != 3 || len(one.Room.Photos) == 0 {
		t.Fatalf("%+v", one.Room)
	}

	if e := s.Do(http.MethodGet, "/v1/rooms/treehouse", nil, nil).Error(t); e.Code != apperr.CodeNotFound {
		t.Fatalf("unknown room: %s", e.Code)
	}

	var prop api.PropertyResponse
	s.Do(http.MethodGet, "/v1/property", nil, nil).JSON(t, &prop)
	if prop.HoldMinutes != 10 || len(prop.Facts) != 12 {
		t.Fatalf("%+v", prop)
	}
}

func TestPriceCalendar(t *testing.T) {
	s := apitest.New(t)
	var cal api.CalendarResponse
	res := s.Do(http.MethodGet, "/v1/rooms/coffee-villa/calendar?from=2026-11-01&to=2026-12-01", nil, nil)
	res.JSON(t, &cal)
	if res.Status != http.StatusOK || len(cal.Days) != 30 {
		t.Fatalf("%d, %d days", res.Status, len(cal.Days))
	}
	byDate := map[string]api.CalendarDayDTO{}
	for _, d := range cal.Days {
		byDate[d.Date] = d
	}
	if d := byDate["2026-11-06"]; d.PricePaise != 2_420_000 || !d.Season || d.MinStay != 2 {
		t.Errorf("Diwali Friday: %+v", d)
	}
	if d := byDate["2026-11-14"]; d.Left != 1 || d.PricePaise != 2_010_000 {
		t.Errorf("14 Nov: %+v", d)
	}
	if e := s.Do(http.MethodGet, "/v1/rooms/coffee-villa/calendar?from=2026-11-01&to=2027-03-01", nil, nil).Error(t); e.Code != apperr.CodeValidation {
		t.Errorf("long range: %s", e.Code)
	}
}

func TestAvailability(t *testing.T) {
	s := apitest.New(t)
	var av api.AvailabilityResponse
	res := s.Do(http.MethodGet, "/v1/availability?check_in=2026-11-14&check_out=2026-11-16&adults=2&children=1", nil, nil)
	res.JSON(t, &av)
	if res.Status != http.StatusOK || av.Stay.Nights != 2 || len(av.Options) != 5 {
		t.Fatalf("%d %+v", res.Status, av.Stay)
	}
	for _, o := range av.Options {
		if o.Slug == "coffee-villa" && (o.Left != 1 || o.Plans[0].Quote.TotalPaise != 4_436_800) {
			t.Errorf("coffee villa: %+v", o)
		}
	}

	e := s.Do(http.MethodGet, "/v1/availability?check_out=2026-11-16&adults=99", nil, nil).Error(t)
	fields, _ := e.Details["fields"].(map[string]any)
	if e.Code != apperr.CodeValidation || fields["check_in"] == nil || fields["adults"] == nil {
		t.Fatalf("expected both problems in one response: %+v", e)
	}
}

func TestHoldLifecycle(t *testing.T) {
	s := apitest.New(t)
	var rooms api.RoomResponse
	s.Do(http.MethodGet, "/v1/rooms/coffee-villa", nil, nil).JSON(t, &rooms)
	var bb api.PlanDTO
	for _, p := range rooms.Room.Plans {
		if p.Code == "BB-FLEX" {
			bb = p
		}
	}
	body := api.CreateHoldRequest{RoomTypeID: rooms.Room.ID, RatePlanID: bb.ID, CheckIn: "2026-11-14", CheckOut: "2026-11-16", Adults: 2, Children: 1}

	var held api.HoldResponse
	res := s.Do(http.MethodPost, "/v1/holds", body, nil)
	res.JSON(t, &held)
	if res.Status != http.StatusCreated || held.Hold.Token == "" || held.Hold.Quote.TotalPaise != 4_720_000 {
		t.Fatalf("%d %s", res.Status, res.Body)
	}

	// The last villa is now held: the next guest gets SOLD_OUT with the nights and nearby dates.
	e := s.Do(http.MethodPost, "/v1/holds", body, nil).Error(t)
	if e.Code != apperr.CodeSoldOut || e.Details["nights"] == nil || e.Details["alternatives"] == nil {
		t.Fatalf("%+v", e)
	}

	path := "/v1/holds/" + held.Hold.ID.String()
	if e := s.Do(http.MethodDelete, path, nil, map[string]string{"X-Hold-Token": "guess"}).Error(t); e.Code != apperr.CodeHoldNotFound {
		t.Fatalf("wrong token: %s", e.Code)
	}
	if res := s.Do(http.MethodDelete, path, nil, map[string]string{"X-Hold-Token": held.Hold.Token}); res.Status != http.StatusNoContent {
		t.Fatalf("release: %d", res.Status)
	}
	if e := s.Do(http.MethodDelete, path, nil, map[string]string{"X-Hold-Token": held.Hold.Token}).Error(t); e.Code != apperr.CodeHoldExpired {
		t.Fatalf("release twice: %s", e.Code)
	}
	if res := s.Do(http.MethodPost, "/v1/holds", body, nil); res.Status != http.StatusCreated {
		t.Fatalf("released villa should be bookable again: %d", res.Status)
	}
}

func TestErrorEnvelopeEverywhere(t *testing.T) {
	s := apitest.New(t)
	cases := []struct {
		name   string
		method string
		path   string
		body   any
		status int
		code   apperr.Code
	}{
		{"unknown route", http.MethodGet, "/v1/nope", nil, http.StatusNotFound, apperr.CodeNotFound},
		{"wrong method", http.MethodPut, "/v1/rooms", nil, http.StatusMethodNotAllowed, apperr.CodeMethodNotAllowed},
		{"unknown field", http.MethodPost, "/v1/holds", map[string]any{"room_type_id": "x", "admin": true}, http.StatusUnprocessableEntity, apperr.CodeValidation},
		{"bad id", http.MethodDelete, "/v1/holds/not-a-uuid", nil, http.StatusUnprocessableEntity, apperr.CodeValidation},
		{"past stay", http.MethodGet, "/v1/availability?check_in=2026-10-01&check_out=2026-10-03", nil, http.StatusUnprocessableEntity, apperr.CodeInvalidStay},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := s.Do(tc.method, tc.path, tc.body, nil)
			if e := res.Error(t); res.Status != tc.status || e.Code != tc.code || e.RequestID == "" {
				t.Fatalf("%d %+v", res.Status, e)
			}
		})
	}
}

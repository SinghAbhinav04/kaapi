package api_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"kaapi/api/internal/inventory"
	"kaapi/api/internal/platform/apperr"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

func ptr[T any](v T) *T { return &v }

func grid(t *testing.T, s *apitest.Server, query string) api.RatesGridResponse {
	t.Helper()
	var g api.RatesGridResponse
	res := s.Do(http.MethodGet, "/v1/staff/rates?"+query, nil, nil)
	if res.Status != http.StatusOK {
		t.Fatalf("grid: %d %s", res.Status, res.Body)
	}
	res.JSON(t, &g)
	return g
}

func night(t *testing.T, g api.RatesGridResponse, slug, date string) api.RateNightDTO {
	t.Helper()
	for _, r := range g.Rows {
		if r.Slug != slug {
			continue
		}
		for _, n := range r.Nights {
			if n.Date == date {
				return n
			}
		}
	}
	t.Fatalf("no %s on %s", slug, date)
	return api.RateNightDTO{}
}

func TestRatesGrid(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "reservations@kaapiestate.test", nil)
	g := grid(t, s, "from=2026-11-09&to=2026-11-23")
	if len(g.Rows) != 5 || len(g.Rows[0].Nights) != 14 {
		t.Fatalf("%d rows", len(g.Rows))
	}
	if n := night(t, g, "coffee-villa", "2026-11-14"); n.Allotment != 2 || n.Sold != 1 || n.Left != 1 || n.PricePaise != 2_010_000 {
		t.Fatalf("%+v", n)
	}
	if n := night(t, g, "pool-villa", "2026-11-21"); n.Left != 0 {
		t.Fatalf("pool villa should be full on 21 Nov: %+v", n)
	}
	bb := grid(t, s, "from=2026-11-14&to=2026-11-15&plan=bb-flex")
	if n := night(t, bb, "coffee-villa", "2026-11-14"); n.PricePaise != 2_130_000 {
		t.Fatalf("breakfast plan price %d", n.PricePaise)
	}
}

func TestWeekendRepriceCascadesToEveryPlan(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "reservations@kaapiestate.test", nil)
	room := roomID(t, s, "coffee-villa")

	var out api.UpdateRatesResponse
	res := s.Do(http.MethodPut, "/v1/staff/rates", api.UpdateRatesRequest{RoomTypeID: room, From: "2026-11-16", To: "2026-11-30",
		Weekdays: []string{"FRI", "SAT"}, RoomOnlyPricePaise: ptr[int64](2_200_000)}, nil)
	res.JSON(t, &out)
	if res.Status != http.StatusOK || out.NightsChanged != 4 { // 20, 21, 27, 28 Nov
		t.Fatalf("%d %s", res.Status, res.Body)
	}
	ro := grid(t, s, "from=2026-11-19&to=2026-11-22")
	if night(t, ro, "coffee-villa", "2026-11-20").PricePaise != 2_200_000 || night(t, ro, "coffee-villa", "2026-11-19").PricePaise != 1_750_000 {
		t.Fatal("only Friday and Saturday should change")
	}
	nrf := grid(t, s, "from=2026-11-20&to=2026-11-21&plan=BB-NRF")
	if got := night(t, nrf, "coffee-villa", "2026-11-20").PricePaise; got != 2_090_000 { // (22,000 + 1,200) × 0.9 = 20,880 → ₹20,900
		t.Fatalf("non-refundable derived %d", got)
	}

	var audits int
	if err := s.Pool.QueryRow(s.Ctx, `SELECT count(*) FROM audit_logs WHERE action = 'rates.bulk_update' AND entity_id = $1`, room.String()).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit rows %d %v", audits, err)
	}
}

func TestAllotmentCannotGoBelowSold(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "gm@kaapiestate.test", nil)
	room := roomID(t, s, "coffee-villa")

	e := s.Do(http.MethodPut, "/v1/staff/rates", api.UpdateRatesRequest{RoomTypeID: room, From: "2026-11-13", To: "2026-11-16", Allotment: ptr(0)}, nil).Error(t)
	if e.Code != apperr.CodeAllotmentTooLow || e.Details["sold"] == nil {
		t.Fatalf("%+v", e)
	}
	if e := s.Do(http.MethodPut, "/v1/staff/rates", api.UpdateRatesRequest{RoomTypeID: room, From: "2026-11-13", To: "2026-11-16", Allotment: ptr(9)}, nil).Error(t); e.Code != apperr.CodeValidation {
		t.Fatalf("more than the 8 keys: %s", e.Code)
	}
	// Weekday filter: closing only Thursday 19 Nov is fine, even though the range has sold nights elsewhere.
	if res := s.Do(http.MethodPut, "/v1/staff/rates", api.UpdateRatesRequest{RoomTypeID: room, From: "2026-11-13", To: "2026-11-20",
		Weekdays: []string{"THU"}, Allotment: ptr(0)}, nil); res.Status != http.StatusOK {
		t.Fatalf("thursday only: %d %s", res.Status, res.Body)
	}
}

func TestStopSellShowsUpInAvailability(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "gm@kaapiestate.test", nil)
	room := roomID(t, s, "plantation-room")
	if res := s.Do(http.MethodPut, "/v1/staff/rates", api.UpdateRatesRequest{RoomTypeID: room, From: "2026-12-07", To: "2026-12-09",
		StopSell: ptr(true), StopSellReason: ptr("Roof repairs")}, nil); res.Status != http.StatusOK {
		t.Fatalf("%d %s", res.Status, res.Body)
	}
	var av api.AvailabilityResponse
	s.NewSession().Do(http.MethodGet, "/v1/availability?check_in=2026-12-07&check_out=2026-12-09", nil, nil).JSON(t, &av)
	for _, o := range av.Options {
		if o.Slug == "plantation-room" && o.Status != inventory.Restricted {
			t.Fatalf("plantation room after stop-sell: %s", o.Status)
		}
	}
}

func TestRateEditPermissions(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "sales@kaapiestate.test", nil)
	room := roomID(t, s, "plantation-room")
	grid(t, s, "from=2026-11-09&to=2026-11-10") // sales can look
	if e := s.Do(http.MethodPut, "/v1/staff/rates", api.UpdateRatesRequest{RoomTypeID: room, From: "2026-11-09", To: "2026-11-10",
		RoomOnlyPricePaise: ptr[int64](100)}, nil).Error(t); e.Code != apperr.CodeForbidden {
		t.Fatalf("sales repricing: %s", e.Code)
	}
	if e := s.NewSession().Do(http.MethodGet, "/v1/staff/rates?from=2026-11-09&to=2026-11-10", nil, nil).Error(t); e.Code != apperr.CodeUnauthenticated {
		t.Fatalf("anonymous: %s", e.Code)
	}
}

func roomID(t *testing.T, s *apitest.Server, slug string) uuid.UUID {
	t.Helper()
	var r api.RoomResponse
	s.Do(http.MethodGet, "/v1/rooms/"+slug, nil, nil).JSON(t, &r)
	return r.Room.ID
}

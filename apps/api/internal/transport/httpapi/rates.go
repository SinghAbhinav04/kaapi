package httpapi

import (
	"net/http"
	"strings"
	"time"

	"kaapi/api/internal/admin"
	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/httpx"
)

// weekdayCodes are the three-letter day names the console sends.
var weekdayCodes = map[string]time.Weekday{
	"MON": time.Monday, "TUE": time.Tuesday, "WED": time.Wednesday, "THU": time.Thursday,
	"FRI": time.Friday, "SAT": time.Saturday, "SUN": time.Sunday,
}

// ratesGrid: GET /v1/staff/rates?from=&to=&plan=
func (a *API) ratesGrid(w http.ResponseWriter, r *http.Request) error {
	p := httpx.NewParams(r)
	stay := domain.Stay{CheckIn: p.Date("from"), CheckOut: p.Date("to")}
	plan := strings.ToUpper(p.String("plan", admin.RoomOnlyPlan))
	if err := p.Err(); err != nil {
		return err
	}
	g, err := a.admin.Grid(r.Context(), stay, plan)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, ratesGridDTO(g))
	return nil
}

// updateRates: PUT /v1/staff/rates. Price edits need rates:write; allotment and
// restrictions need inventory:write.
func (a *API) updateRates(w http.ResponseWriter, r *http.Request) error {
	var body UpdateRatesRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	fields := map[string]string{}
	c := admin.Change{
		RoomTypeID:         body.RoomTypeID,
		Stay:               domain.Stay{CheckIn: httpx.ParseDate(fields, "from", body.From), CheckOut: httpx.ParseDate(fields, "to", body.To)},
		RoomOnlyPricePaise: body.RoomOnlyPricePaise, Allotment: body.Allotment, StopSell: body.StopSell,
		StopSellReason: body.StopSellReason, MinStay: body.MinStay, NoArrival: body.NoArrival,
	}
	for _, d := range body.Weekdays {
		wd, ok := weekdayCodes[strings.ToUpper(d)]
		if !ok {
			fields["weekdays"] = "must be day codes like MON, FRI"
			break
		}
		c.Weekdays = append(c.Weekdays, wd)
	}
	if len(fields) > 0 {
		return apperr.Validation(fields)
	}

	p := principal(r.Context())
	if (c.TouchesPrice() && !auth.Can(p.Role, auth.PermRatesWrite)) || (c.TouchesInventory() && !auth.Can(p.Role, auth.PermInventoryWrite)) {
		return auth.ErrForbidden
	}
	n, err := a.admin.Apply(r.Context(), p.UserID, c)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, UpdateRatesResponse{NightsChanged: n})
	return nil
}

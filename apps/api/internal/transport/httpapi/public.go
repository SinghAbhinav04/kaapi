package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/httpx"
)

// Guest-facing limits for query parameters.
const (
	maxRoomsQuery    = 4
	maxAdultsQuery   = 16
	maxChildrenQuery = 8
	defaultPlanCode  = "RO-FLEX"
	headerHoldToken  = "X-Hold-Token"
)

func (a *API) property(w http.ResponseWriter, r *http.Request) error {
	p, err := a.catalog.Property(r.Context())
	if err != nil {
		return err
	}
	out := PropertyResponse{Name: p.Name, CheckInTime: p.CheckInTime, CheckOutTime: p.CheckOutTime, HoldMinutes: p.HoldMinutes,
		Facts: make([]FactDTO, len(p.Facts))}
	for i, f := range p.Facts {
		out.Facts[i] = FactDTO{Key: f.Key, Category: f.Category, Question: f.Question, Answer: f.Answer}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (a *API) listRooms(w http.ResponseWriter, r *http.Request) error {
	rooms, err := a.catalog.Rooms(r.Context())
	if err != nil {
		return err
	}
	out := RoomsResponse{Rooms: make([]RoomDTO, len(rooms))}
	for i, rt := range rooms {
		out.Rooms[i] = roomDTO(rt)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (a *API) getRoom(w http.ResponseWriter, r *http.Request) error {
	room, err := a.catalog.Room(r.Context(), r.PathValue("slug"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, RoomResponse{Room: roomDTO(room)})
	return nil
}

// roomCalendar: GET /v1/rooms/{slug}/calendar?from=&to=&plan=
func (a *API) roomCalendar(w http.ResponseWriter, r *http.Request) error {
	p := httpx.NewParams(r)
	from, to := p.Date("from"), p.Date("to")
	plan := strings.ToUpper(p.String("plan", defaultPlanCode))
	if err := p.Err(); err != nil {
		return err
	}
	days, err := a.catalog.Calendar(r.Context(), r.PathValue("slug"), plan, from, to)
	if err != nil {
		return err
	}
	out := CalendarResponse{Plan: plan, Days: make([]CalendarDayDTO, len(days))}
	for i, d := range days {
		out.Days[i] = CalendarDayDTO{Date: date(d.Date), PricePaise: d.PricePaise, Left: d.Left, MinStay: d.MinStay,
			StopSell: d.StopSell, NoArrival: d.NoArrival, Season: d.Season}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// availability: GET /v1/availability?check_in=&check_out=&adults=&children=&rooms=&promo=
func (a *API) availability(w http.ResponseWriter, r *http.Request) error {
	p := httpx.NewParams(r)
	stay := domain.Stay{CheckIn: p.Date("check_in"), CheckOut: p.Date("check_out")}
	party := domain.Party{
		Rooms:    p.Int("rooms", 1, 1, maxRoomsQuery),
		Adults:   p.Int("adults", 2, 1, maxAdultsQuery),
		Children: p.Int("children", 0, 0, maxChildrenQuery),
	}
	promo := p.String("promo", "")
	if err := p.Err(); err != nil {
		return err
	}
	opts, err := a.inventory.Search(r.Context(), inventory.SearchRequest{Stay: stay, Party: party, PromoCode: promo})
	if err != nil {
		return err
	}
	out := AvailabilityResponse{Stay: stayDTO(stay), Options: make([]OptionDTO, len(opts))}
	for i, o := range opts {
		out.Options[i] = optionDTO(o)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// createHold: POST /v1/holds. When the stay is sold out, the error carries nearby dates
// that would work, so the page can offer them instead of a dead end.
func (a *API) createHold(w http.ResponseWriter, r *http.Request) error {
	var body CreateHoldRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	fields := map[string]string{}
	stay := domain.Stay{CheckIn: httpx.ParseDate(fields, "check_in", body.CheckIn), CheckOut: httpx.ParseDate(fields, "check_out", body.CheckOut)}
	if body.RoomTypeID == uuid.Nil {
		fields["room_type_id"] = "is required"
	}
	if body.RatePlanID == uuid.Nil {
		fields["rate_plan_id"] = "is required"
	}
	if len(fields) > 0 {
		return apperr.Validation(fields)
	}
	if body.Rooms == 0 {
		body.Rooms = 1
	}
	hold, err := a.inventory.CreateHold(r.Context(), inventory.HoldRequest{
		RoomTypeID: body.RoomTypeID, RatePlanID: body.RatePlanID, Stay: stay,
		Party: domain.Party{Rooms: body.Rooms, Adults: body.Adults, Children: body.Children}, PromoCode: body.PromoCode,
	})
	var soldOut *inventory.SoldOutError
	if errors.As(err, &soldOut) {
		ae := soldOut.AppError()
		if alts, aerr := a.inventory.Alternatives(r.Context(), body.RoomTypeID, stay.CheckIn, stay.Nights(), body.Rooms); aerr == nil {
			ae.Details["alternatives"] = domain.FormatDates(alts)
		}
		return ae
	}
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, HoldResponse{Hold: holdDTO(hold, true)})
	return nil
}

// releaseHold: DELETE /v1/holds/{id}, with the hold's token in X-Hold-Token.
func (a *API) releaseHold(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := a.inventory.VerifyToken(r.Context(), id, r.Header.Get(headerHoldToken)); err != nil {
		return err
	}
	if err := a.inventory.Release(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// health: GET /healthz (process up) and /readyz (database reachable).
func (a *API) healthz(w http.ResponseWriter, _ *http.Request) error {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}

func (a *API) readyz(w http.ResponseWriter, r *http.Request) error {
	if err := a.pool.Ping(r.Context()); err != nil {
		return apperr.Wrap(err, apperr.CodeUnavailable, "The database isn't reachable.")
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	return nil
}

// fxRates: GET /v1/fx-rates → today's rates for the "≈" estimate. Empty when they're stale.
func (a *API) fxRates(w http.ResponseWriter, r *http.Request) error {
	rates, err := a.fx.Rates(r.Context())
	if err != nil {
		return err
	}
	out := FxRatesResponse{ChargedIn: domain.CurrencyINR, Rates: make([]FxRateDTO, len(rates))}
	for i, x := range rates {
		out.Rates[i] = FxRateDTO{Currency: x.Currency, INRPerUnitMicros: x.INRPerUnitMicros, Date: date(x.Date)}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

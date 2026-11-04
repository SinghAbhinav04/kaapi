package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"kaapi/api/internal/admin"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/httpx"
)

// listOffers: GET /v1/staff/offers
func (a *API) listOffers(w http.ResponseWriter, r *http.Request) error {
	offers, err := a.admin.Offers(r.Context())
	if err != nil {
		return err
	}
	out := OffersResponse{Offers: make([]OfferDTO, len(offers))}
	for i, o := range offers {
		out.Offers[i] = offerDTO(o)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// createOffer: POST /v1/staff/offers
func (a *API) createOffer(w http.ResponseWriter, r *http.Request) error {
	in, err := decodeOffer(w, r)
	if err != nil {
		return err
	}
	o, err := a.admin.CreateOffer(r.Context(), principal(r.Context()).UserID, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, OfferResponse{Offer: offerDTO(o)})
	return nil
}

// updateOffer: PUT /v1/staff/offers/{id}
func (a *API) updateOffer(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	in, err := decodeOffer(w, r)
	if err != nil {
		return err
	}
	o, err := a.admin.UpdateOffer(r.Context(), principal(r.Context()).UserID, id, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, OfferResponse{Offer: offerDTO(o)})
	return nil
}

func decodeOffer(w http.ResponseWriter, r *http.Request) (admin.OfferInput, error) {
	var body OfferRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return admin.OfferInput{}, err
	}
	fields := map[string]string{}
	in := admin.OfferInput{Code: body.Code, Description: body.Description, Type: body.Type, Value: body.Value,
		StayFrom: httpx.ParseDate(fields, "stay_from", body.StayFrom), StayTo: httpx.ParseDate(fields, "stay_to", body.StayTo),
		BookFrom: httpx.ParseDate(fields, "book_from", body.BookFrom), BookTo: httpx.ParseDate(fields, "book_to", body.BookTo),
		MinNights: body.MinNights, MaxUses: body.MaxUses, RoomTypeIDs: body.RoomTypeIDs, AutoApply: body.AutoApply, Active: body.Active, Announce: body.Announce}
	if len(fields) > 0 {
		return in, apperr.Validation(fields)
	}
	return in, nil
}

func offerDTO(o admin.Offer) OfferDTO {
	if o.RoomTypeIDs == nil {
		o.RoomTypeIDs = []uuid.UUID{} // "every room type" is an empty list on the wire, never null
	}
	return OfferDTO{OfferRequest: OfferRequest{Code: o.Code, Description: o.Description, Type: o.Type, Value: o.Value,
		StayFrom: date(o.StayFrom), StayTo: date(o.StayTo), BookFrom: date(o.BookFrom), BookTo: date(o.BookTo),
		MinNights: o.MinNights, MaxUses: o.MaxUses, RoomTypeIDs: o.RoomTypeIDs, AutoApply: o.AutoApply, Active: o.Active},
		ID: o.ID, UsedCount: o.UsedCount, UpdatedAt: o.UpdatedAt.UTC().Truncate(time.Second)}
}

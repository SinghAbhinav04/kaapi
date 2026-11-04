package httpapi

import (
	"net/http"

	"kaapi/api/internal/campaign"
	"kaapi/api/internal/platform/httpx"
)

const campaignsListed = 50

// listCampaigns: GET /v1/staff/campaigns → recent campaigns and how many guests receive them.
func (a *API) listCampaigns(w http.ResponseWriter, r *http.Request) error {
	list, subscribers, err := a.campaigns.List(r.Context(), campaignsListed)
	if err != nil {
		return err
	}
	out := CampaignsResponse{Campaigns: make([]CampaignDTO, len(list)), Subscribers: subscribers}
	for i, c := range list {
		out.Campaigns[i] = campaignDTO(c)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// createCampaign: POST /v1/staff/campaigns — a festive or announcement email to opted-in guests.
func (a *API) createCampaign(w http.ResponseWriter, r *http.Request) error {
	var body CampaignRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	c, err := a.campaigns.Create(r.Context(), principal(r.Context()).UserID,
		campaign.Input{Kind: campaign.Kind(body.Kind), Subject: body.Subject, Body: body.Body, SendAt: body.SendAt})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, CampaignResponse{Campaign: campaignDTO(c)})
	return nil
}

// unsubscribe: POST /v1/marketing/unsubscribe — the link in every offers email.
func (a *API) unsubscribe(w http.ResponseWriter, r *http.Request) error {
	var body UnsubscribeRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	if err := a.campaigns.Unsubscribe(r.Context(), body.Token); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func campaignDTO(c campaign.Campaign) CampaignDTO {
	return CampaignDTO{ID: c.ID, Kind: string(c.Kind), Subject: c.Subject, Body: c.Body, Status: string(c.Status),
		SendAt: c.SendAt, SentAt: c.SentAt, Recipients: c.Recipients, CreatedAt: c.CreatedAt}
}

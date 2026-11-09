package api_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/apperr"
	api "kaapi/api/internal/transport/httpapi"
	"kaapi/api/tests/apitest"
)

var unsubscribeRe = regexp.MustCompile(`/unsubscribe\?token=([0-9a-f]{32})`)

// Offers reach exactly the guests who asked for them, each email once, and one click stops them.
func TestOffersReachOnlyGuestsWhoOptedIn(t *testing.T) {
	s := apitest.New(t)

	// The guest ticks "send me offers" at checkout.
	h := holdVilla(t, s)
	res := s.Do(http.MethodPost, "/v1/bookings", api.CreateBookingRequest{HoldID: h.ID, Guest: guest, Consent: true, MarketingOptIn: true},
		map[string]string{"X-Hold-Token": h.Token})
	if res.Status != http.StatusCreated {
		t.Fatalf("book: %d %s", res.Status, res.Body)
	}
	// Someone who signed in but never opted in must get nothing.
	guestLogin(t, s.NewSession(), "quiet@example.test")

	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	var list api.CampaignsResponse
	gm.Do(http.MethodGet, "/v1/staff/campaigns", nil, nil).JSON(t, &list)
	if list.Subscribers != 1 {
		t.Fatalf("subscribers %d, want 1", list.Subscribers)
	}

	// Publishing an offer with "announce" notifies the campaign watcher in the same transaction.
	offer := diwali()
	offer.Announce = true
	var created api.OfferResponse
	gm.Do(http.MethodPost, "/v1/staff/offers", offer, nil).JSON(t, &created)
	if n, err := s.App.Campaigns.Dispatch(s.Ctx); err != nil || n != 1 {
		t.Fatalf("dispatch queued %d (err %v), want 1", n, err)
	}
	mails := outboxTo(t, s, domain.OutboxPromotion, guest.Email)
	// An automatic offer needs no code, and the email says so rather than asking for one.
	if len(mails) != 1 || !strings.Contains(mails[0].Body, "no code needed") || strings.Contains(mails[0].Body, "Use code") ||
		mails[0].Status != domain.OutboxQueued {
		t.Fatalf("offer email: %+v", mails)
	}
	if got := outboxTo(t, s, domain.OutboxPromotion, "quiet@example.test"); len(got) != 0 {
		t.Fatalf("not opted in, yet emailed: %+v", got)
	}

	// Editing the offer later never announces it again.
	offer.Description = "Diwali on the estate: 15% off, now with breakfast"
	gm.Do(http.MethodPut, "/v1/staff/offers/"+created.Offer.ID.String(), offer, nil).JSON(t, &created)
	if n, _ := s.App.Campaigns.Dispatch(s.Ctx); n != 0 {
		t.Fatalf("re-announced on edit: %d", n)
	}

	// A festive email written by staff goes the same way.
	festive := api.CampaignRequest{Kind: "FESTIVE", Subject: "Christmas in the coffee hills", Body: "Plum cake, bonfires and misty mornings this December."}
	if res := gm.Do(http.MethodPost, "/v1/staff/campaigns", festive, nil); res.Status != http.StatusCreated {
		t.Fatalf("create campaign: %d %s", res.Status, res.Body)
	}
	if n, _ := s.App.Campaigns.Dispatch(s.Ctx); n != 1 {
		t.Fatalf("festive dispatch %d, want 1", n)
	}

	// One click unsubscribes; after that, nothing more arrives.
	m := unsubscribeRe.FindStringSubmatch(outboxTo(t, s, domain.OutboxPromotion, guest.Email)[0].Body)
	if m == nil {
		t.Fatal("no unsubscribe link in the email")
	}
	if res := s.NewSession().Do(http.MethodPost, "/v1/marketing/unsubscribe", api.UnsubscribeRequest{Token: m[1]}, nil); res.Status != http.StatusNoContent {
		t.Fatalf("unsubscribe: %d %s", res.Status, res.Body)
	}
	gm.Do(http.MethodPost, "/v1/staff/campaigns", festive, nil)
	if n, _ := s.App.Campaigns.Dispatch(s.Ctx); n != 0 {
		t.Fatalf("unsubscribed, yet queued %d", n)
	}
	if e := s.NewSession().Do(http.MethodPost, "/v1/marketing/unsubscribe", api.UnsubscribeRequest{Token: "nope"}, nil).Error(t); e.Code != apperr.CodeNotFound {
		t.Fatalf("bad token: %s", e.Code)
	}
}

func TestCampaignsNeedTheOffersPermission(t *testing.T) {
	s := apitest.New(t)
	staffLogin(t, s, "reservations@kaapiestate.test", nil)
	if res := s.Do(http.MethodGet, "/v1/staff/campaigns", nil, nil); res.Status != http.StatusForbidden {
		t.Fatalf("reservations: %d", res.Status)
	}
	gm := s.NewSession()
	staffLogin(t, gm, "gm@kaapiestate.test", nil)
	if e := gm.Do(http.MethodPost, "/v1/staff/campaigns", api.CampaignRequest{Kind: "OFFER", Subject: "Hi there", Body: "Some body text here."}, nil).Error(t); e.Code != apperr.CodeValidation {
		t.Fatalf("offer kind must come from an offer: %s", e.Code)
	}
}

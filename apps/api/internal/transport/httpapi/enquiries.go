package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/enquiry"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/httpx"
	"kaapi/api/internal/platform/money"
)

// enquiryKind is the public form's two shapes.
const (
	enquiryKindGeneral = "GENERAL"
	enquiryKindEvent   = "EVENT"
)

// ownLanes is which lanes a role works when it can't see every lane.
var ownLanes = map[domain.Role][]domain.Lane{
	domain.RoleReservations: {domain.LaneReservations},
	domain.RoleSales:        {domain.LaneSales},
}

// lanesFor returns nil for "every lane": granted by permission, otherwise the caller's team lane.
func (a *API) lanesFor(r *http.Request) []domain.Lane {
	if a.can(r, auth.PermEnquiriesAll) {
		return nil
	}
	if l, ok := ownLanes[principal(r.Context()).Role]; ok {
		return l
	}
	return []domain.Lane{} // a team with no lane of its own sees none
}

func (a *API) canSeeLane(r *http.Request, lane domain.Lane) bool {
	l := a.lanesFor(r)
	return l == nil || slices.Contains(l, lane)
}

// createEnquiry: POST /v1/enquiries, the public web and event forms.
func (a *API) createEnquiry(w http.ResponseWriter, r *http.Request) error {
	var body CreateEnquiryRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	in := enquiry.IntakeInput{Channel: domain.ChannelWebForm, SenderName: body.Name, SenderEmail: body.Email,
		SenderPhone: body.Phone, Text: body.Message}
	if strings.EqualFold(body.Kind, enquiryKindEvent) && body.Event != nil {
		in.Channel = domain.ChannelEventForm
		in.Subject = "Event enquiry"
		in.Text = eventText(*body.Event, body.Message)
	}
	ref, err := a.enquiries.Intake(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, EnquiryCreatedResponse{Ref: ref})
	return nil
}

// eventText turns the structured event form into the message the pipeline reads, so the
// AI and the keyword rules see the same facts the guest typed.
func eventText(e EventDetailsDTO, note string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Event enquiry: %s", strings.ToLower(e.Type))
	if e.Date != "" {
		fmt.Fprintf(&b, " on %s", e.Date)
	} else if e.Month != "" {
		fmt.Fprintf(&b, " in %s", e.Month)
	}
	if e.GuestCount > 0 {
		fmt.Fprintf(&b, ", about %d guests", e.GuestCount)
	}
	if e.Nights > 0 {
		fmt.Fprintf(&b, ", %d nights", e.Nights)
	}
	if e.BudgetPaise > 0 {
		fmt.Fprintf(&b, ", budget around %s", money.FormatINR(e.BudgetPaise))
	}
	b.WriteString(".")
	if note = strings.TrimSpace(note); note != "" {
		b.WriteString("\n\n" + note)
	}
	return b.String()
}

// simulateInbound: POST /v1/staff/inbound. Email and WhatsApp are simulated in this build;
// staff paste a message here and it enters the same pipeline a real webhook would feed.
func (a *API) simulateInbound(w http.ResponseWriter, r *http.Request) error {
	var body InboundMessageRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	ch := domain.Channel(strings.ToUpper(body.Channel))
	if ch != domain.ChannelEmail && ch != domain.ChannelWhatsApp {
		return apperr.Validation(map[string]string{"channel": "must be EMAIL or WHATSAPP"})
	}
	ref, err := a.enquiries.Intake(r.Context(), enquiry.IntakeInput{Channel: ch, SenderName: body.SenderName,
		SenderEmail: body.SenderEmail, SenderPhone: body.SenderPhone, Subject: body.Subject, Text: body.Text})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, EnquiryCreatedResponse{Ref: ref})
	return nil
}

// listEnquiries: GET /v1/staff/enquiries?lane=&status=&q=&limit=
func (a *API) listEnquiries(w http.ResponseWriter, r *http.Request) error {
	p := httpx.NewParams(r)
	f := enquiry.ListFilter{Status: domain.EnquiryStatus(strings.ToUpper(p.String("status", ""))), Query: p.String("q", ""),
		Limit: p.Int("limit", enquiry.DefaultListLimit, 1, enquiry.MaxListLimit)}
	lane := domain.Lane(strings.ToUpper(p.String("lane", "")))
	if err := p.Err(); err != nil {
		return err
	}
	switch {
	case lane != "" && !lane.Valid():
		return enquiry.ErrBadLane
	case lane != "" && !a.canSeeLane(r, lane):
		return auth.ErrForbidden
	case lane != "":
		f.Lanes = []domain.Lane{lane}
	default:
		f.Lanes = a.lanesFor(r)
	}
	if f.Status != "" && !f.Status.Valid() {
		return apperr.Validation(map[string]string{"status": "is not an enquiry status"})
	}
	list, err := a.enquiries.List(r.Context(), f)
	if err != nil {
		return err
	}
	out := EnquiriesResponse{Enquiries: make([]EnquirySummaryDTO, len(list))}
	for i, e := range list {
		out.Enquiries[i] = enquirySummaryDTO(e)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// getEnquiry: GET /v1/staff/enquiries/{ref}. Outside your lanes it doesn't exist.
func (a *API) getEnquiry(w http.ResponseWriter, r *http.Request) error {
	e, err := a.visibleEnquiry(r)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, EnquiryResponse{Enquiry: enquiryDTO(e)})
	return nil
}

func (a *API) visibleEnquiry(r *http.Request) (enquiry.Enquiry, error) {
	e, err := a.enquiries.Get(r.Context(), r.PathValue("ref"), principal(r.Context()).UserID)
	if err != nil {
		return e, err
	}
	if !a.canSeeLane(r, e.Lane) {
		return enquiry.Enquiry{}, enquiry.ErrNotFound
	}
	return e, nil
}

// routeEnquiry: POST /v1/staff/enquiries/{ref}/route, the one-click re-route.
func (a *API) routeEnquiry(w http.ResponseWriter, r *http.Request) error {
	var body RouteEnquiryRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	if _, err := a.visibleEnquiry(r); err != nil {
		return err
	}
	var cat *domain.Category
	if body.Category != "" {
		c := domain.Category(strings.ToUpper(body.Category))
		cat = &c
	}
	if err := a.enquiries.Reroute(r.Context(), r.PathValue("ref"), principal(r.Context()).UserID,
		domain.Lane(strings.ToUpper(body.Lane)), cat); err != nil {
		return err
	}
	return a.writeEnquiry(w, r)
}

// correctEnquiry: PATCH /v1/staff/enquiries/{ref}/fields fixes what the AI extracted.
func (a *API) correctEnquiry(w http.ResponseWriter, r *http.Request) error {
	var body CorrectEnquiryRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	if _, err := a.visibleEnquiry(r); err != nil {
		return err
	}
	fields := map[string]string{}
	c := enquiry.Corrections{Adults: body.Adults, Children: body.Children, Rooms: body.Rooms, GuestCount: body.GuestCount}
	if body.CheckIn != nil {
		d := httpx.ParseDate(fields, "check_in", *body.CheckIn)
		c.CheckIn = &d
	}
	if body.CheckOut != nil {
		d := httpx.ParseDate(fields, "check_out", *body.CheckOut)
		c.CheckOut = &d
	}
	if len(fields) > 0 {
		return apperr.Validation(fields)
	}
	if err := a.enquiries.Correct(r.Context(), r.PathValue("ref"), principal(r.Context()).UserID, c); err != nil {
		return err
	}
	return a.writeEnquiry(w, r)
}

// editDraft: PUT /v1/staff/enquiries/{ref}/draft
func (a *API) editDraft(w http.ResponseWriter, r *http.Request) error {
	var body EditDraftRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	if _, err := a.visibleEnquiry(r); err != nil {
		return err
	}
	if err := a.enquiries.EditDraft(r.Context(), r.PathValue("ref"), principal(r.Context()).UserID, body.Body); err != nil {
		return err
	}
	return a.writeEnquiry(w, r)
}

// sendReply: POST /v1/staff/enquiries/{ref}/send. Nothing is ever sent without this.
func (a *API) sendReply(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.visibleEnquiry(r); err != nil {
		return err
	}
	if err := a.enquiries.Send(r.Context(), r.PathValue("ref"), principal(r.Context()).UserID); err != nil {
		return err
	}
	return a.writeEnquiry(w, r)
}

// setEnquiryStatus: POST /v1/staff/enquiries/{ref}/status
func (a *API) setEnquiryStatus(w http.ResponseWriter, r *http.Request) error {
	var body EnquiryStatusRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return err
	}
	if _, err := a.visibleEnquiry(r); err != nil {
		return err
	}
	if err := a.enquiries.SetStatus(r.Context(), r.PathValue("ref"), principal(r.Context()).UserID,
		domain.EnquiryStatus(strings.ToUpper(body.Status))); err != nil {
		return err
	}
	return a.writeEnquiry(w, r)
}

// retryAI: POST /v1/staff/enquiries/{ref}/retry-ai
func (a *API) retryAI(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.visibleEnquiry(r); err != nil {
		return err
	}
	if err := a.enquiries.RetryAI(r.Context(), r.PathValue("ref")); err != nil {
		return err
	}
	return a.writeEnquiry(w, r)
}

// dashboard: GET /v1/staff/dashboard
func (a *API) dashboard(w http.ResponseWriter, r *http.Request) error {
	st, err := a.enquiries.Stats(r.Context())
	if err != nil {
		return err
	}
	desk, err := a.bookings.Desk(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, dashboardDTO(st, desk, a.enquiries.AIEnabled()))
	return nil
}

// inboxCounts: GET /v1/staff/inbox/counts: open enquiries per lane the caller can see, and the
// week's median first reply when they can see the dashboard. What the sidebar shows, without
// downloading the inbox; it refetches on the live streams, never on a timer.
func (a *API) inboxCounts(w http.ResponseWriter, r *http.Request) error {
	lanes := a.lanesFor(r)
	counts, err := a.enquiries.LaneCounts(r.Context(), lanes)
	if err != nil {
		return err
	}
	out := InboxCountsResponse{Lanes: []LaneCountDTO{}}
	for _, lane := range domain.Lanes {
		if lanes == nil || slices.Contains(lanes, lane) {
			out.Lanes = append(out.Lanes, LaneCountDTO{Lane: lane, Count: counts[lane]})
		}
	}
	if a.can(r, auth.PermDashboardRead) {
		median, err := a.enquiries.MedianFirstReply(r.Context())
		if err != nil {
			return err
		}
		out.ReplyTime = &ReplyTimeDTO{MedianMinutes: median, TargetMinutes: firstReplyTargetMinutes}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (a *API) writeEnquiry(w http.ResponseWriter, r *http.Request) error {
	e, err := a.enquiries.Get(r.Context(), r.PathValue("ref"), principal(r.Context()).UserID)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, EnquiryResponse{Enquiry: enquiryDTO(e)})
	return nil
}

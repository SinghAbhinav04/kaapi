package api

import (
	"net/http"

	"kaapi/api/internal/analytics"
	"kaapi/api/internal/platform/apperr"
	"kaapi/api/internal/platform/httpx"
)

// analyticsReport: GET /v1/staff/analytics?from=&to= → revenue, pace, mix and what's on the
// books. The range is the booked date (IST); it defaults to the last 90 days.
func (a *API) analyticsReport(w http.ResponseWriter, r *http.Request) error {
	p := httpx.NewParams(r)
	to := a.analytics.Today()
	from := to.AddDate(0, 0, -(analytics.DefaultDays - 1))
	if r.URL.Query().Has("to") {
		to = p.Date("to")
	}
	if r.URL.Query().Has("from") {
		from = p.Date("from")
	}
	if err := p.Err(); err != nil {
		return err
	}
	if to.Before(from) {
		return apperr.Validation(map[string]string{"to": "must be on or after from"})
	}
	if to.Sub(from).Hours()/24 >= analytics.MaxDays {
		return apperr.Validation(map[string]string{"from": "the range can be at most two years"})
	}
	rep, err := a.analytics.Report(r.Context(), from, to)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, analyticsDTO(rep))
	return nil
}

func analyticsDTO(r analytics.Report) AnalyticsResponse {
	k := r.KPIs
	out := AnalyticsResponse{
		Range: AnalyticsRangeDTO{From: date(r.From), To: date(r.To)},
		KPIs: AnalyticsKPIsDTO{RevenuePaise: k.RevenuePaise, RoomRevenuePaise: k.RoomRevenuePaise, GSTPaise: k.GSTPaise,
			Bookings: k.Bookings, RoomNights: k.RoomNights, ADRPaise: k.ADRPaise, AvgStayNights: k.AvgStayNights,
			AvgLeadDays: k.AvgLeadDays, CancellationRate: k.CancellationRate, CommissionSavedPaise: k.CommissionSavedPaise,
			AvgBookingValuePaise: k.AvgBookingValuePaise, PreviousRevenuePaise: k.PreviousRevenuePaise,
			PreviousBookings: k.PreviousBookingsCount},
		Daily:      make([]AnalyticsDayDTO, len(r.Daily)),
		ByRoom:     breakdownDTOs(r.ByRoom),
		ByPlan:     breakdownDTOs(r.ByPlan),
		ByCurrency: breakdownDTOs(r.ByCurrency),
		StatusMix:  countDTOs(r.StatusMix),
		LeadTime:   countDTOs(r.LeadTime),
		OnTheBooks: make([]AnalyticsMonthDTO, len(r.OnTheBooks)),
		Enquiries: AnalyticsEnquiriesDTO{Total: r.Enquiries.Total, ByChannel: countDTOs(r.Enquiries.ByChannel),
			ByCategory: countDTOs(r.Enquiries.ByCategory), Converted: r.Enquiries.Converted,
			MedianFirstReplyMinutes: r.Enquiries.MedianFirstReplyMinutes, AIHandledRate: r.Enquiries.AIHandledRate},
	}
	for i, d := range r.Daily {
		out.Daily[i] = AnalyticsDayDTO{Date: date(d.Date), Bookings: d.Bookings, RevenuePaise: d.RevenuePaise}
	}
	for i, m := range r.OnTheBooks {
		out.OnTheBooks[i] = AnalyticsMonthDTO{Month: m.Month.Format("2006-01"), RoomNights: m.RoomNights,
			RoomRevenuePaise: m.RoomRevenuePaise, Allotment: m.Allotment, Sold: m.Sold}
	}
	return out
}

func breakdownDTOs(in []analytics.Breakdown) []AnalyticsBreakdownDTO {
	out := make([]AnalyticsBreakdownDTO, len(in))
	for i, b := range in {
		out[i] = AnalyticsBreakdownDTO{Name: b.Name, Bookings: b.Bookings, RoomNights: b.RoomNights, RevenuePaise: b.RevenuePaise}
	}
	return out
}

func countDTOs(in []analytics.Count) []AnalyticsCountDTO {
	out := make([]AnalyticsCountDTO, len(in))
	for i, c := range in {
		out[i] = AnalyticsCountDTO{Key: c.Key, Count: c.Count}
	}
	return out
}

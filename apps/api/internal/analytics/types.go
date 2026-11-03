package analytics

import "time"

const (
	// DefaultDays is the booked-date window when the caller names none.
	DefaultDays = 90
	// MaxDays bounds a request so one call can't scan years of bookings.
	MaxDays = 731
	// AheadMonths is how far "on the books" looks forward.
	AheadMonths = 6

	// agencyCommissionBps: the brief says agencies take about 20%. Every direct booking keeps that
	// share of room revenue; shown as an estimate, labelled as an assumption.
	agencyCommissionBps = 2000
	bpsDenominator      = 10_000
)

// Lead-time buckets in days before arrival, ending with an open "90+".
var leadBuckets = []struct {
	Label    string
	Min, Max int // Max < 0: no upper bound
}{
	{"0–7 days", 0, 7}, {"8–30 days", 8, 30}, {"31–60 days", 31, 60}, {"61–90 days", 61, 90}, {"90+ days", 91, -1},
}

// Report is everything the analytics tab draws, for bookings made in [From, To].
type Report struct {
	From, To   time.Time
	KPIs       KPIs
	Daily      []Day
	ByRoom     []Breakdown
	ByPlan     []Breakdown
	StatusMix  []Count
	LeadTime   []Count
	OnTheBooks []Month
	Enquiries  Enquiries
}

// KPIs count confirmed bookings (archived ones included). Cancellations are everything that
// was paid for and then given back or called off.
type KPIs struct {
	RevenuePaise          int64 // total paid, GST included
	RoomRevenuePaise      int64 // after discount, before GST
	GSTPaise              int64
	Bookings              int
	RoomNights            int
	ADRPaise              int64   // room revenue per room night
	AvgStayNights         float64 // per booking
	AvgLeadDays           float64 // booked to arrival
	CancellationRate      *float64
	CommissionSavedPaise  int64
	AvgBookingValuePaise  int64
	PreviousRevenuePaise  int64 // same-length window just before From, for the trend
	PreviousBookingsCount int
}

type Day struct {
	Date         time.Time
	Bookings     int
	RevenuePaise int64
}

type Breakdown struct {
	Name         string
	Bookings     int
	RoomNights   int
	RevenuePaise int64
}

type Count struct {
	Key   string
	Count int
}

// Month is one stay month ahead: what's already sold against the website's allotment.
type Month struct {
	Month            time.Time // first of the month
	RoomNights       int
	RoomRevenuePaise int64
	Allotment        int
	Sold             int
}

type Enquiries struct {
	Total                   int
	ByChannel               []Count
	ByCategory              []Count
	Converted               int // linked to a booking
	MedianFirstReplyMinutes *float64
	AIHandledRate           *float64 // read by a model rather than the keyword fallback
}

package retention

import "time"

// Policy. Every window here is a labelled assumption in the TDD; the privacy page states the same.
const (
	// TaxRetentionFY is how many full financial years (1 April to 31 March) an archived
	// booking is kept after the year its stay ended in. 8 follows the Companies Act, 2013
	// s.128(5) (books of account), which outlasts GST's 72 months (CGST Act s.36).
	TaxRetentionFY = 8

	// Archived bookings lose the guest's contact details this long after check-out. The name,
	// dates, amounts and GST stay: they are the tax record.
	contactRetention = 12 // months

	// Enquiries, sent-message copies and staff alerts are deleted after this, linked or not:
	// the booking is the record, not the conversation about it.
	messageRetention = 12 // months

	unpaidRetention  = 90 * 24 * time.Hour // bookings that never took money
	holdRetention    = 7 * 24 * time.Hour  // expired or released holds no booking points to
	otpRetention     = 24 * time.Hour
	sessionRetention = 30 * 24 * time.Hour
	bucketRetention  = 24 * time.Hour // rate-limit windows

	// batchSize bounds each transaction, so the sweep never holds many row locks at once.
	batchSize = 500

	auditAction = "retention.sweep"
	auditEntity = "retention"
)

// Result counts what one sweep changed.
type Result struct {
	Archived      int64 `json:"archived"`
	Anonymised    int64 `json:"anonymised"`
	PurgedTax     int64 `json:"purged_after_tax_window"`
	PurgedUnpaid  int64 `json:"purged_unpaid"`
	Enquiries     int64 `json:"enquiries"`
	Outbox        int64 `json:"outbox"`
	Notifications int64 `json:"notifications"`
	Holds         int64 `json:"holds"`
	OTPs          int64 `json:"otps"`
	Sessions      int64 `json:"sessions"`
	Housekeeping  int64 `json:"housekeeping"` // rate-limit windows and spent idempotency keys
}

// Total is the number of rows touched.
func (r Result) Total() int64 {
	return r.Archived + r.Anonymised + r.PurgedTax + r.PurgedUnpaid + r.Enquiries + r.Outbox + r.Notifications +
		r.Holds + r.OTPs + r.Sessions + r.Housekeeping
}

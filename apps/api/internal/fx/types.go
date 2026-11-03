// Package fx keeps the exchange rates used to show a guest an estimate in their own currency.
// Nothing is ever charged in these currencies: the quote, the payment and the GST invoice are
// INR, and a rate only turns an INR amount into "≈ US$102" for reading.
package fx

import (
	"time"

	"kaapi/api/internal/domain"
)

const (
	// MicrosPerUnit scales a rate: INR per one unit of the currency, times a million
	// (84.123456 INR per USD is stored as 84123456), so rates are integers like money.
	MicrosPerUnit = 1_000_000

	// FreshFor is how long a fetched rate is shown. The ECB publishes once a working day and
	// we fetch every few hours, so a rate this old means fetching has been failing: better to
	// show INR only than an estimate that has drifted.
	FreshFor = 48 * time.Hour

	// Frankfurter serves the ECB reference rates (free, no key). Tests point at a fake.
	defaultBaseURL = "https://api.frankfurter.dev/v1"
	ecbBase        = domain.CurrencyEUR
	requestTimeout = 15 * time.Second
	maxBodyBytes   = 64 << 10
)

// Rate is INR per one unit of Currency, as published on Date.
type Rate struct {
	Currency         domain.Currency
	INRPerUnitMicros int64
	Date             time.Time // ECB publication date (a calendar date, midnight UTC)
	FetchedAt        time.Time
}

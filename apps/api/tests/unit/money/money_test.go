package money_test

import (
	"testing"

	"kaapi/api/internal/platform/money"
)

func TestFormatINRUsesIndianGrouping(t *testing.T) {
	cases := map[int64]string{
		0:            "₹0",
		50:           "₹0.50",
		1_634_400:    "₹16,344",
		4_248_000:    "₹42,480",
		12_000_000:   "₹1,20,000",
		1_234_567_89: "₹12,34,567.89",
		-200_000:     "−₹2,000",
	}
	for paise, want := range cases {
		if got := money.FormatINR(paise); got != want {
			t.Errorf("%d: %q, want %q", paise, got, want)
		}
	}
}

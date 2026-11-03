package retention_test

import (
	"testing"
	"time"

	"kaapi/api/internal/retention"
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestFinancialYearStart(t *testing.T) {
	for in, want := range map[string]string{
		"2026-04-01": "2026-04-01", // first day of a year
		"2026-11-02": "2026-04-01",
		"2027-03-31": "2026-04-01", // last day still belongs to the year that began the April before
		"2027-01-15": "2026-04-01",
	} {
		if got := retention.FinancialYearStart(day(in)).Format(time.DateOnly); got != want {
			t.Errorf("FinancialYearStart(%s) = %s, want %s", in, got, want)
		}
	}
}

// A stay in FY 2026-27 is kept for 8 full years after it: through 31 Mar 2035, gone from 1 Apr 2035.
func TestPurgeBeforeKeepsEightFullYears(t *testing.T) {
	stay := day("2026-11-14")
	cases := []struct {
		today string
		purge bool
	}{
		{"2035-03-31", false},
		{"2035-04-01", true},
		{"2030-01-01", false},
	}
	for _, c := range cases {
		cut := retention.PurgeBefore(day(c.today), retention.TaxRetentionFY)
		if got := stay.Before(cut); got != c.purge {
			t.Errorf("on %s: purge a %s stay = %v, want %v (cut-off %s)", c.today, stay.Format(time.DateOnly), got, c.purge, cut.Format(time.DateOnly))
		}
	}
}

package retention

import "time"

// FinancialYearStart is 1 April of the Indian financial year that d falls in.
func FinancialYearStart(d time.Time) time.Time {
	y := d.Year()
	if d.Month() < time.April {
		y--
	}
	return time.Date(y, time.April, 1, 0, 0, 0, 0, time.UTC)
}

// PurgeBefore is the check-out date before which an archived booking has been kept for
// `years` full financial years after the one it fell in. A stay in FY 2026-27 (ending
// 31 Mar 2027) with 8 years is kept to 31 Mar 2035 and may go from 1 Apr 2035.
func PurgeBefore(today time.Time, years int) time.Time {
	return FinancialYearStart(today.AddDate(-years, 0, 0))
}

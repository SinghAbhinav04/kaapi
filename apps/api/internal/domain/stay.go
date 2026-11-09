package domain

import "time"

// Stay is a check-in and check-out pair of calendar dates. Dates carry no time of day:
// they are midnight UTC values that mean "this date at the resort".
type Stay struct {
	CheckIn  time.Time
	CheckOut time.Time
}

// Nights is the number of nights between check-in and check-out.
func (s Stay) Nights() int { return NightsBetween(s.CheckIn, s.CheckOut) }

// LastNight is the date of the final night (the day before check-out).
func (s Stay) LastNight() time.Time { return s.CheckOut.AddDate(0, 0, -1) }

// NightsBetween counts nights between two calendar dates.
func NightsBetween(in, out time.Time) int { return int(out.Sub(in).Hours() / 24) }

// Party is who is staying: how many rooms, and the guests across them.
type Party struct {
	Rooms    int
	Adults   int
	Children int
}

// FormatDates renders dates as YYYY-MM-DD, the API's date format.
func FormatDates(ds []time.Time) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Format(time.DateOnly)
	}
	return out
}

// resortZone is the estate's time zone. The property settings carry the same default; this
// is for code that has no settings at hand (reminders, the AI's dates, the staff's Today page).
var resortZone = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Kolkata"); err == nil {
		return loc
	}
	return time.FixedZone("IST", 5*3600+1800)
}()

// ResortZone is the estate's time zone.
func ResortZone() *time.Location { return resortZone }

// ResortToday is the estate's calendar date at t, as a stay date (midnight UTC).
func ResortToday(t time.Time) time.Time {
	l := t.In(resortZone)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}

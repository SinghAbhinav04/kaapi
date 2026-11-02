package idgen

import (
	"regexp"
	"testing"
)

// Same pattern as the CHECK constraint on bookings.ref.
var refPattern = regexp.MustCompile(`^KS-[2-9A-HJ-NP-Z]{6}$`)

func TestBookingRefMatchesTheDatabaseRule(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		r := BookingRef()
		if !refPattern.MatchString(r) {
			t.Fatalf("%q doesn't match the bookings.ref CHECK", r)
		}
		seen[r] = true
	}
	if len(seen) < 1990 {
		t.Fatalf("too many repeats: %d unique of 2000", len(seen))
	}
}

func TestNewIsTimeOrdered(t *testing.T) {
	a, b := New(), New()
	if a.Version() != 7 || a.String() >= b.String() {
		t.Fatalf("%s then %s", a, b)
	}
}

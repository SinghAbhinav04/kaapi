// Package idgen creates identifiers: time-ordered UUIDs for rows, and short booking
// references that guests can read out over the phone.
package idgen

import (
	"crypto/rand"
	"math/big"

	"github.com/google/uuid"
)

const (
	// BookingRefPrefix and the alphabet match the CHECK on bookings.ref.
	BookingRefPrefix = "KS-"
	bookingRefLen    = 6
	// No 0/O or 1/I, so a reference can't be misread.
	bookingRefAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

// New returns a UUIDv7: unique and sortable by creation time, which keeps indexes compact.
func New() uuid.UUID { return uuid.Must(uuid.NewV7()) }

// BookingRef returns e.g. "KS-7Q3M9X": 32^6 ≈ 1.07 billion combinations. The unique index on
// bookings.ref is the final guard; callers retry on a collision.
func BookingRef() string {
	b := make([]byte, bookingRefLen)
	max := big.NewInt(int64(len(bookingRefAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err) // the OS random source failing is not recoverable
		}
		b[i] = bookingRefAlphabet[n.Int64()]
	}
	return BookingRefPrefix + string(b)
}

// Package money formats paise for people. Calculations never leave int64 paise.
package money

import (
	"strconv"
	"strings"
)

const paisePerRupee = 100

// FormatINR renders paise the Indian way: ₹1,20,000 or ₹44,368.50.
func FormatINR(paise int64) string {
	neg := paise < 0
	if neg {
		paise = -paise
	}
	rupees, rem := paise/paisePerRupee, paise%paisePerRupee
	digits := strconv.FormatInt(rupees, 10)
	// Last three digits, then groups of two: 12,34,56,789.
	var groups []string
	if len(digits) > 3 {
		head, tail := digits[:len(digits)-3], digits[len(digits)-3:]
		for len(head) > 2 {
			groups = append([]string{head[len(head)-2:]}, groups...)
			head = head[:len(head)-2]
		}
		groups = append([]string{head}, groups...)
		digits = strings.Join(groups, ",") + "," + tail
	}
	out := "₹" + digits
	if rem != 0 {
		out += "." + strconv.FormatInt(rem+paisePerRupee, 10)[1:]
	}
	if neg {
		out = "−" + out
	}
	return out
}

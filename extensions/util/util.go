package util

import (
	"math"
)

// FiniteOrZero turns anything that is not a real number into zero.
//
// It was FiniteOrZero, and it let through the case that actually happens:
// dividing by a count of zero gives +Inf, and only 0/0 gives NaN. Nearly
// 50,000 of nopCommerce's 82,558 co-change rows carried "+Inf" as a
// percentage, which every reader downstream then had to parse as a number
// and could not.
func FiniteOrZero(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

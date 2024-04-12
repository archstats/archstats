package util

import (
	"math"
)

func NanToZero(value float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	return value
}

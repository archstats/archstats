package views

import "math"

func nanToZero(value float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	return value
}

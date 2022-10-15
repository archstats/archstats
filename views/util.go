package views

import (
	"math"
)

func nanToZero(value float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	return value
}

func groupBy[T any](connections []T, groupBy func(connection T) string) map[string][]T {
	toReturn := make(map[string][]T)
	for _, connection := range connections {
		group := groupBy(connection)
		toReturn[group] = append(toReturn[group], connection)
	}
	return toReturn
}

func mapTo[X any, Y any](before []X, after func(before X) Y) []Y {
	toReturn := make([]Y, 0, len(before))
	for _, item := range before {
		toReturn = append(toReturn, after(item))
	}
	return toReturn
}

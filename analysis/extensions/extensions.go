package extensions

import (
	"github.com/RyanSusana/archstats/analysis/extensions/regex"
)

func BuiltInExtensions() []string {
	return regex.AvailableExtensions()
}

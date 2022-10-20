package extensions

import "github.com/RyanSusana/archstats/extensions/regex"

func BuiltInExtensions() []string {
	return regex.AvailableExtensions()
}

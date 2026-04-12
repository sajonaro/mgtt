package render

import "fmt"

// Deterministic is set by tests to fix timestamps/IDs so output is stable.
var Deterministic bool

// Checkmark returns "✓" if ok is true, "✗" otherwise.
func Checkmark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

// Pluralize returns "N singular" when n == 1, or "N plural" otherwise.
func Pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

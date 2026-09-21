// Package authorname normalizes personal names between "Last, First" and
// "First Last" ordering.
package authorname

import "strings"

// Normalize converts a "Last, First" formatted name to "First Last". Names
// without a comma are returned unchanged. Names with two or more commas
// (suffixes like "King, Martin Luther, Jr") are also returned unchanged
// rather than flipped wrong.
func Normalize(name string) string {
	last, first, found := strings.Cut(name, ",")
	if !found || strings.Contains(first, ",") {
		return name
	}
	last = strings.TrimSpace(last)
	first = strings.TrimSpace(first)
	if last == "" || first == "" {
		return name
	}
	return first + " " + last
}

// NormalizeAll applies Normalize to every element of names.
func NormalizeAll(names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = Normalize(name)
	}
	return out
}

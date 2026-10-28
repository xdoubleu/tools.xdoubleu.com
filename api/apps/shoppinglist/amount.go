package shoppinglist

import (
	"strconv"
	"strings"
)

// toAmount formats f without rounding, trimming trailing zeros; capped at 3
// decimals to match the web formatter and absorb float noise.
func toAmount(f float64) string {
	if f <= 0 {
		return "0"
	}
	s := strconv.FormatFloat(f, 'f', 3, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

package recipes

import (
	"fmt"
	"math"
)

const eighths = 8

//nolint:gochecknoglobals //package-level lookup table, not mutable state
var fractionSymbols = map[int]string{
	0: "",
	1: "⅛",
	2: "¼",
	3: "⅜",
	4: "½",
	5: "⅝",
	6: "¾",
	7: "⅞",
}

// ToFraction converts a float64 to a Unicode cooking fraction string (nearest 1/8th).
func ToFraction(f float64) string {
	if f <= 0 {
		return "0"
	}
	whole := int(math.Floor(f))
	nearest := int(math.Round((f - float64(whole)) * eighths))
	if nearest == eighths {
		whole++
		nearest = 0
	}
	fracStr := fractionSymbols[nearest]
	if whole == 0 {
		if fracStr == "" {
			return "0"
		}
		return fracStr
	}
	return fmt.Sprintf("%d%s", whole, fracStr)
}

// toFraction is a package-level alias for internal use.
func toFraction(f float64) string {
	return ToFraction(f)
}

package recipes

import (
	"fmt"
	"math"
)

type fracEntry struct {
	val    float64
	symbol string
}

//nolint:gochecknoglobals //package-level lookup table, not mutable state
var commonFractions = []fracEntry{
	{0.0, ""},
	{1.0 / 8, "⅛"},
	{1.0 / 4, "¼"},
	{1.0 / 3, "⅓"},
	{3.0 / 8, "⅜"},
	{1.0 / 2, "½"},
	{5.0 / 8, "⅝"},
	{2.0 / 3, "⅔"},
	{3.0 / 4, "¾"},
	{7.0 / 8, "⅞"},
	{1.0, ""},
}

// ToFraction converts a float64 to a Unicode cooking fraction string.
func ToFraction(f float64) string {
	if f <= 0 {
		return "0"
	}
	whole := int(math.Floor(f))
	frac := f - float64(whole)

	bestDiff := math.MaxFloat64
	bestIdx := 0
	for i, cf := range commonFractions {
		if diff := math.Abs(frac - cf.val); diff <= bestDiff {
			bestDiff = diff
			bestIdx = i
		}
	}
	symbol := commonFractions[bestIdx].symbol
	if bestIdx == len(commonFractions)-1 {
		whole++
		symbol = ""
	}

	if whole == 0 {
		if symbol == "" {
			return "0"
		}
		return symbol
	}
	if symbol == "" {
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d%s", whole, symbol)
}

// toFraction is a package-level alias for internal use.
func toFraction(f float64) string {
	return ToFraction(f)
}

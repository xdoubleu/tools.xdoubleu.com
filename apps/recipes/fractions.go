package recipes

import "tools.xdoubleu.com/internal/templates"

// toFraction is a package-level alias so handler code can call it directly.
func toFraction(f float64) string {
	return templates.ToFraction(f)
}

package mealplans

import "strings"

// customItemSep separates a hand-typed item's name from its optional amount
// within a single line of a custom meal's newline-separated custom_name, e.g.
// "apples\t2". A tab is used because it cannot be typed into a single-line input.
const customItemSep = "\t"

// displayCustomName renders a custom meal's newline-separated items for human
// display, turning each "name\tamount" line into "amount name" and dropping the
// raw tab separator. Lines without an amount are returned unchanged.
func displayCustomName(customName string) string {
	lines := strings.Split(customName, "\n")
	for i, line := range lines {
		name, amount, found := strings.Cut(line, customItemSep)
		if found && strings.TrimSpace(amount) != "" {
			lines[i] = strings.TrimSpace(amount) + " " + name
		} else {
			lines[i] = name
		}
	}
	return strings.Join(lines, "\n")
}

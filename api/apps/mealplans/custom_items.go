package mealplans

import "strings"

// customItemSep separates a custom item's name from its amount ("apples\t2");
// a tab can't be typed into the single-line input.
const customItemSep = "\t"

// displayCustomName renders "name\tamount" lines as "amount name".
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

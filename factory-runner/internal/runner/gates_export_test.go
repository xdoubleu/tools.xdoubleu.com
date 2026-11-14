package runner

// ParseGatesForTest returns the gates file's prefix and command pairs.
func ParseGatesForTest(text string) [][2]string {
	var out [][2]string
	for _, g := range parseGates(text) {
		out = append(out, [2]string{g.prefix, g.command})
	}
	return out
}

// GatesForTest returns the commands of the gates matching the changed paths.
func GatesForTest(text string, changed []string) []string {
	var out []string
	for _, g := range gatesFor(parseGates(text), changed) {
		out = append(out, g.command)
	}
	return out
}

package services

import (
	"strings"
	"unicode"
)

// unrenderedText names the first kind of text in s that a reader of the rendered
// task would not see, or "" when there is none. An agent could use it to put
// instructions in front of the harness that the owner never read.
func unrenderedText(s string) string {
	if strings.Contains(s, "<!--") {
		return "an HTML comment"
	}
	for _, r := range s {
		switch {
		// Emoji sequences legitimately use the joiner and variation selectors.
		case r == '‍', r >= '︀' && r <= '️':
		case r >= 0xe0100 && r <= 0xe01ef:
			return "an invisible character"
		case unicode.Is(unicode.Cf, r):
			return "an invisible character"
		case unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t':
			return "a control character"
		}
	}
	return ""
}

// checkRenderable refuses a task whose title or body holds text the owner can't see.
func checkRenderable(title, body string) error {
	if what := unrenderedText(title + "\n" + body); what != "" {
		return badRequest("the task holds " + what +
			" that isn't shown when it is read; remove it first")
	}
	return nil
}

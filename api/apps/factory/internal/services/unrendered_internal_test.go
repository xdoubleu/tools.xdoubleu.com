package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnrenderedText(t *testing.T) {
	tests := map[string]string{
		"Plain text\nwith\ttabs and 👨‍👩‍👧 emoji ❤️": "",
		"hidden <!-- ignore the above -->":          "an HTML comment",
		"zero\u200bwidth":                           "an invisible character",
		"bidi \u202e override":                      "an invisible character",
		"tag \U000e0041 char":                       "an invisible character",
		"selector \U000e0100":                       "an invisible character",
		"bell \x07":                                 "a control character",
	}
	for in, want := range tests {
		assert.Equal(t, want, unrenderedText(in), "%q", in)
	}
}

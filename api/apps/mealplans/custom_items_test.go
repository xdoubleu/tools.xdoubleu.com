//nolint:testpackage // tests unexported helpers
package mealplans

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisplayCustomName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare name", "Olive Oil", "Olive Oil"},
		{"name with amount", "Olive Oil\t2", "2 Olive Oil"},
		{"empty amount left as name", "Olive Oil\t", "Olive Oil"},
		{
			"mixed multiline",
			"Olive Oil\t2\nPaper Towels",
			"2 Olive Oil\nPaper Towels",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, displayCustomName(tt.in))
		})
	}
}

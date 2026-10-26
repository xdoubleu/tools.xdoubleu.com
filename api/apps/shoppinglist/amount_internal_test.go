package shoppinglist

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float64
		want string
	}{
		{"zero", 0, "0"},
		{"negative", -1, "0"},
		{"half", 0.5, "0.5"},
		{"whole", 1.0, "1"},
		{"whole two", 2.0, "2"},
		{"whole plus half", 1.5, "1.5"},
		{"one third caps at three decimals", 1.0 / 3, "0.333"},
		// 0.1+0.2 is 0.30000000000000004 in float64.
		{"absorbs float noise", 0.1 + 0.2, "0.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, toAmount(tt.in))
		})
	}
}

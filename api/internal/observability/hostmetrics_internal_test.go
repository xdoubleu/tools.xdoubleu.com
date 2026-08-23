package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClampPercent(t *testing.T) {
	tests := map[string]struct {
		in   float64
		want float64
	}{
		"below zero clamps to zero":   {in: -5, want: 0},
		"above full clamps to full":   {in: 150, want: fullPercent},
		"within range passes through": {in: 42.5, want: 42.5},
		"exactly zero passes through": {in: 0, want: 0},
		"exactly full passes through": {in: fullPercent, want: fullPercent},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.InDelta(t, tt.want, clampPercent(tt.in), 0.001)
		})
	}
}

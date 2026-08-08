package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/convert"
)

func TestAnyToString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   any
		want    string
		wantErr bool
	}{
		{"string", "hello", "hello", false},
		{"bool", true, "true", false},
		{"int", 1, "1", false},
		{"int64", int64(2), "2", false},
		{"float64", 1.5, "1.50", false},
		{"string slice", []string{"a", "b"}, "a,b", false},
		{"int slice", []int{1, 2}, "1,2", false},
		{"unsupported", struct{}{}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := convert.AnyToString(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

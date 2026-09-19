package authorname_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/pkg/authorname"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "flips lastname, firstname",
			input: "Tolkien, J.R.R.",
			want:  "J.R.R. Tolkien",
		},
		{
			name:  "leaves already-correct name unchanged",
			input: "J.R.R. Tolkien",
			want:  "J.R.R. Tolkien",
		},
		{
			name:  "leaves a suffix-bearing name with two commas unchanged",
			input: "King, Martin Luther, Jr",
			want:  "King, Martin Luther, Jr",
		},
		{
			name:  "trims whitespace around the comma",
			input: "Tolkien ,  J.R.R.",
			want:  "J.R.R. Tolkien",
		},
		{
			name:  "leaves an empty string unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "leaves a name with an empty last segment unchanged",
			input: ", J.R.R.",
			want:  ", J.R.R.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, authorname.Normalize(tt.input))
		})
	}
}

func TestNormalizeAll(t *testing.T) {
	t.Parallel()

	got := authorname.NormalizeAll(
		[]string{"Tolkien, J.R.R.", "George Orwell"},
	)
	assert.Equal(t, []string{"J.R.R. Tolkien", "George Orwell"}, got)
}

package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSections(t *testing.T) {
	s, err := parseSections(`{"summary": "S", "know": ["a","b","c","d","e","f","g"],
		"like": [], "drifting": ["x"], "bounce": null}`)
	require.NoError(t, err)
	assert.Equal(t, "S", s.Summary)
	assert.Len(t, s.Know, maxBullets)
	assert.Empty(t, s.Like)
	assert.Equal(t, []string{"x"}, s.Drifting)
	assert.Empty(t, s.Bounce)

	_, err = parseSections(`[`)
	require.ErrorContains(t, err, "not valid JSON")
	_, err = parseSections(`{"summary": " "}`)
	require.ErrorContains(t, err, "no summary")
}

func TestErrorText(t *testing.T) {
	long := make([]byte, maxErrorLength+50)
	for i := range long {
		long[i] = 'x'
	}
	got := errorText(errString(string(long)))
	assert.Len(t, got, maxErrorLength+len("…"))
	assert.Equal(t, "short", errorText(errString("short")))
}

type errString string

func (e errString) Error() string { return string(e) }

func TestProfileSchemaRequiresEverySection(t *testing.T) {
	s := profileSchema()
	assert.Equal(t, false, s["additionalProperties"])
	assert.ElementsMatch(t,
		[]string{"summary", "know", "like", "drifting", "bounce"}, s["required"])
}

//nolint:testpackage // testing unexported package-level helpers
package connect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToggleTag_AddNew(t *testing.T) {
	result := toggleTag([]string{"a", "b"}, "c", true)
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestToggleTag_Remove(t *testing.T) {
	result := toggleTag([]string{"a", "b", "c"}, "b", false)
	assert.Equal(t, []string{"a", "c"}, result)
}

func TestToggleTag_EnableAlreadyPresent(t *testing.T) {
	result := toggleTag([]string{"a", "b"}, "a", true)
	assert.Equal(t, []string{"b", "a"}, result)
}

func TestToggleTag_RemoveAbsent(t *testing.T) {
	result := toggleTag([]string{"a", "b"}, "z", false)
	assert.Equal(t, []string{"a", "b"}, result)
}

func TestToggleTag_Empty(t *testing.T) {
	result := toggleTag(nil, "tag", true)
	assert.Equal(t, []string{"tag"}, result)
}

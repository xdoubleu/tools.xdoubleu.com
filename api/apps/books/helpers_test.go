//nolint:testpackage // testing unexported package-level helpers
package books

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/internal/models"
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

func TestGroupByStatus_Empty(t *testing.T) {
	shelves := groupByStatus(nil)
	assert.Empty(t, shelves)
}

func TestGroupByStatus_SkipsStandardStatuses(t *testing.T) {
	books := []models.UserBook{
		{Status: models.StatusToRead},  //nolint:exhaustruct //only Status needed
		{Status: models.StatusReading}, //nolint:exhaustruct //only Status needed
		{Status: models.StatusRead},    //nolint:exhaustruct //only Status needed
		{Status: models.StatusDropped}, //nolint:exhaustruct //only Status needed
	}
	shelves := groupByStatus(books)
	assert.Empty(t, shelves)
}

func TestGroupByStatus_CustomStatusBecomesShelf(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	books := []models.UserBook{
		{ //nolint:exhaustruct //only ID and Status needed
			ID:     id1,
			Status: "favorites",
		},
		{ //nolint:exhaustruct //only ID and Status needed
			ID:     id2,
			Status: "favorites",
		},
		{ //nolint:exhaustruct //only ID and Status needed
			ID:     uuid.New(),
			Status: "abandoned",
		},
	}
	shelves := groupByStatus(books)

	assert.Len(t, shelves, 2)
	assert.Equal(t, "abandoned", shelves[0].Name)
	assert.Len(t, shelves[0].Books, 1)
	assert.Equal(t, "favorites", shelves[1].Name)
	assert.Len(t, shelves[1].Books, 2)
}

//nolint:testpackage // testing unexported package-level helpers
package backlog

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/backlog/internal/models"
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

func TestGroupByTags_Empty(t *testing.T) {
	shelves := groupByTags(nil)
	assert.Empty(t, shelves)
}

func TestGroupByTags_SkipsSpecialTags(t *testing.T) {
	books := []models.UserBook{
		{ //nolint:exhaustruct //only Tags needed
			Tags: []string{models.TagOwnPhysical, models.TagFavourite},
		},
	}
	shelves := groupByTags(books)
	assert.Empty(t, shelves)
}

func TestGroupByTags_GroupsAndSorts(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	books := []models.UserBook{
		{ //nolint:exhaustruct //only Tags needed
			ID:   id1,
			Tags: []string{"sci-fi", "classics"},
		},
		{ //nolint:exhaustruct //only Tags needed
			ID:   id2,
			Tags: []string{"classics"},
		},
	}
	shelves := groupByTags(books)

	assert.Len(t, shelves, 2)
	// Sorted alphabetically: "classics" before "sci-fi"
	assert.Equal(t, "classics", shelves[0].Name)
	assert.Len(t, shelves[0].Books, 2)
	assert.Equal(t, "sci-fi", shelves[1].Name)
	assert.Len(t, shelves[1].Books, 1)
}

func TestGroupByTags_MixedSpecialAndNormal(t *testing.T) {
	books := []models.UserBook{
		{ //nolint:exhaustruct //only Tags needed
			Tags: []string{models.TagOwnPhysical, "fantasy"},
		},
	}
	shelves := groupByTags(books)
	assert.Len(t, shelves, 1)
	assert.Equal(t, "fantasy", shelves[0].Name)
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

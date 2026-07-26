//nolint:testpackage // testing unexported package-level helpers
package reading

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/reading/internal/models"
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

// Dropped/owned have no dedicated LibraryResponse field, so groupByStatus
// always surfaces them as empty shelves regardless of registry state — even
// for a library that never touched either status and has no reading.shelves
// row for them (issue #593: a shelf already emptied before the shelves
// registry existed has no user_books row left to register it from, so
// depending on the registry alone can never recover it).
func TestGroupByStatus_Empty(t *testing.T) {
	shelves := groupByStatus(nil, nil)
	assert.Len(t, shelves, 2)
	assert.Equal(t, "dropped", shelves[0].Name)
	assert.Empty(t, shelves[0].Books)
	assert.Equal(t, "owned", shelves[1].Name)
	assert.Empty(t, shelves[1].Books)
}

func TestGroupByStatus_SkipsStandardStatuses(t *testing.T) {
	books := []models.UserBook{
		{Status: models.StatusToRead},  //nolint:exhaustruct //only Status needed
		{Status: models.StatusReading}, //nolint:exhaustruct //only Status needed
		{Status: models.StatusRead},    //nolint:exhaustruct //only Status needed
	}
	shelves := groupByStatus(books, nil)
	assert.Len(t, shelves, 2)
	assert.Equal(t, "dropped", shelves[0].Name)
	assert.Equal(t, "owned", shelves[1].Name)
}

// A registered shelf with no books currently on it must still surface as an
// empty shelf, rather than disappearing because groupByStatus only derives
// shelves from books it sees.
func TestGroupByStatus_RegisteredEmptyShelfSurfaces(t *testing.T) {
	shelves := groupByStatus(nil, []string{"backlog"})
	assert.Len(t, shelves, 3)
	assert.Equal(t, "backlog", shelves[0].Name)
	assert.Empty(t, shelves[0].Books)
	assert.Equal(t, "dropped", shelves[1].Name)
	assert.Equal(t, "owned", shelves[2].Name)
}

// A registered shelf that also has books must not be duplicated — the
// book-derived entry wins and keeps its reading.
func TestGroupByStatus_RegisteredShelfWithBooksNotDuplicated(t *testing.T) {
	books := []models.UserBook{
		{Status: "favorites"}, //nolint:exhaustruct //only Status needed
	}
	shelves := groupByStatus(books, []string{"favorites"})
	assert.Len(t, shelves, 3)
	assert.Equal(t, "dropped", shelves[0].Name)
	assert.Equal(t, "favorites", shelves[1].Name)
	assert.Len(t, shelves[1].Books, 1)
	assert.Equal(t, "owned", shelves[2].Name)
}

// Dropped has no dedicated LibraryResponse field, so unlike the other three
// built-in statuses it flows through groupByStatus as a shelf named
// "dropped" rather than disappearing from the library.
func TestGroupByStatus_DroppedBecomesShelf(t *testing.T) {
	books := []models.UserBook{
		{Status: models.StatusDropped}, //nolint:exhaustruct //only Status needed
		{Status: models.StatusToRead},  //nolint:exhaustruct //only Status needed
	}
	shelves := groupByStatus(books, nil)
	assert.Len(t, shelves, 2)
	assert.Equal(t, "dropped", shelves[0].Name)
	assert.Len(t, shelves[0].Books, 1)
	assert.Equal(t, "owned", shelves[1].Name)
	assert.Empty(t, shelves[1].Books)
}

// Owned has no dedicated LibraryResponse field either, so it flows through
// groupByStatus the same way dropped does.
func TestGroupByStatus_OwnedBecomesShelf(t *testing.T) {
	books := []models.UserBook{
		{Status: models.StatusOwned},  //nolint:exhaustruct //only Status needed
		{Status: models.StatusToRead}, //nolint:exhaustruct //only Status needed
	}
	shelves := groupByStatus(books, nil)
	assert.Len(t, shelves, 2)
	assert.Equal(t, "dropped", shelves[0].Name)
	assert.Empty(t, shelves[0].Books)
	assert.Equal(t, "owned", shelves[1].Name)
	assert.Len(t, shelves[1].Books, 1)
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
	shelves := groupByStatus(books, nil)

	assert.Len(t, shelves, 4)
	assert.Equal(t, "abandoned", shelves[0].Name)
	assert.Len(t, shelves[0].Books, 1)
	assert.Equal(t, "dropped", shelves[1].Name)
	assert.Equal(t, "favorites", shelves[2].Name)
	assert.Len(t, shelves[2].Books, 2)
	assert.Equal(t, "owned", shelves[3].Name)
}

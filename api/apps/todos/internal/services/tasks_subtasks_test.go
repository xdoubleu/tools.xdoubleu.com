//nolint:testpackage // tests unexported helpers
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

// ── buildSubtaskTree ──────────────────────────────────────────────────────────

func TestBuildSubtaskTree_Empty(t *testing.T) {
	result := buildSubtaskTree(nil)
	assert.Empty(t, result)
}

func TestBuildSubtaskTree_FlatTopLevel(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	flat := []models.Subtask{
		{ID: id1}, //nolint:exhaustruct // test fields only
		{ID: id2}, //nolint:exhaustruct // test fields only
	}
	result := buildSubtaskTree(flat)
	assert.Len(t, result, 2)
	assert.Empty(t, result[0].Children)
	assert.Empty(t, result[1].Children)
}

func TestBuildSubtaskTree_OneChild(t *testing.T) {
	parentID := uuid.New()
	childID := uuid.New()
	flat := []models.Subtask{
		{ID: parentID}, //nolint:exhaustruct // test fields only
		{ //nolint:exhaustruct // test fields only
			ID:              childID,
			ParentSubtaskID: &parentID,
		},
	}
	result := buildSubtaskTree(flat)
	assert.Len(t, result, 1)
	assert.Equal(t, parentID, result[0].ID)
	assert.Len(t, result[0].Children, 1)
	assert.Equal(t, childID, result[0].Children[0].ID)
}

func TestBuildSubtaskTree_TwoLevels(t *testing.T) {
	parentID := uuid.New()
	childID := uuid.New()
	grandID := uuid.New()
	flat := []models.Subtask{
		{ID: parentID}, //nolint:exhaustruct // test fields only
		{ //nolint:exhaustruct // test fields only
			ID:              childID,
			ParentSubtaskID: &parentID,
		},
		{ //nolint:exhaustruct // test fields only
			ID:              grandID,
			ParentSubtaskID: &childID,
		},
	}
	result := buildSubtaskTree(flat)
	assert.Len(t, result, 1)
	assert.Len(t, result[0].Children, 1)
	assert.Len(t, result[0].Children[0].Children, 1)
	assert.Equal(t, grandID, result[0].Children[0].Children[0].ID)
}

// ── countSubtasksRecursive ────────────────────────────────────────────────────

func TestCountSubtasksRecursive_Empty(t *testing.T) {
	assert.Equal(t, 0, countSubtasksRecursive(nil))
}

func TestCountSubtasksRecursive_FlatList(t *testing.T) {
	subtasks := []models.Subtask{
		{}, //nolint:exhaustruct // test fields only
		{}, //nolint:exhaustruct // test fields only
	}
	assert.Equal(t, 2, countSubtasksRecursive(subtasks))
}

func TestCountSubtasksRecursive_Nested(t *testing.T) {
	subtasks := []models.Subtask{
		{ //nolint:exhaustruct // test fields only
			Children: []models.Subtask{
				{}, //nolint:exhaustruct // test fields only
				{}, //nolint:exhaustruct // test fields only
			},
		},
	}
	assert.Equal(t, 3, countSubtasksRecursive(subtasks))
}

// ── countDoneSubtasksRecursive ────────────────────────────────────────────────

func TestCountDoneSubtasksRecursive_Empty(t *testing.T) {
	assert.Equal(t, 0, countDoneSubtasksRecursive(nil))
}

func TestCountDoneSubtasksRecursive_NoneComplete(t *testing.T) {
	subtasks := []models.Subtask{
		{Done: false}, //nolint:exhaustruct // test fields only
		{Done: false}, //nolint:exhaustruct // test fields only
	}
	assert.Equal(t, 0, countDoneSubtasksRecursive(subtasks))
}

func TestCountDoneSubtasksRecursive_SomeDone(t *testing.T) {
	subtasks := []models.Subtask{
		{Done: true},  //nolint:exhaustruct // test fields only
		{Done: false}, //nolint:exhaustruct // test fields only
		{ //nolint:exhaustruct // test fields only
			Done: true,
			Children: []models.Subtask{
				{Done: true},  //nolint:exhaustruct // test fields only
				{Done: false}, //nolint:exhaustruct // test fields only
			},
		},
	}
	assert.Equal(t, 3, countDoneSubtasksRecursive(subtasks))
}

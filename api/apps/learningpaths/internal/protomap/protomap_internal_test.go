//nolint:exhaustruct // sparse fixtures
package protomap

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func TestQuizFromProto_NilSkipped(t *testing.T) {
	got := quizFromProto([]*learningpathsv1.QuizQuestion{
		{Prompt: "Q", Options: []string{"a", "b"}, CorrectAnswerIndex: 1},
		nil,
	})
	require.Len(t, got, 2)
	assert.Equal(t, "Q", got[0].Prompt)
	assert.Equal(t, 1, got[0].CorrectAnswerIndex)
	assert.Empty(t, got[1].Prompt)
}

func TestModulesFromProto_ExternalBookAndDue(t *testing.T) {
	modules := ModulesFromProto([]*learningpathsv1.Module{{Items: []*learningpathsv1.Item{
		{Description: "owned"},
		{
			Description: "unowned",
			Due:         "every day at 20:00",
			ExternalBook: &learningpathsv1.ExternalBookRef{
				Provider: "hardcover", ProviderId: "978",
			},
		},
		nil,
	}}, nil})

	items := modules[0].Items
	assert.Nil(t, items[0].ExternalBook)
	require.NotNil(t, items[1].ExternalBook)
	assert.Equal(t, "978", items[1].ExternalBook.ProviderID)
	assert.Equal(t, "every day at 20:00", protoItem(&items[1]).Due)
	assert.Empty(t, items[2].Description)
	assert.Empty(t, modules[1].Title)
}

func TestExternalBookFromProto_IncompleteIgnored(t *testing.T) {
	ref := &learningpathsv1.ExternalBookRef{Provider: "hardcover"}
	assert.Nil(t, externalBookFromProto(ref))
	assert.Nil(t, externalBookFromProto(nil))
}

func TestResourcesFromProto_BadLinkIDIsAbsent(t *testing.T) {
	good, bad := uuid.NewString(), "nope"
	res := ResourcesFromProto([]*learningpathsv1.Resource{
		{Text: "a", LinkedBookId: &good, LinkedFeedItemId: &bad},
		nil,
	})
	require.NotNil(t, res[0].LinkedBookID)
	assert.Equal(t, good, res[0].LinkedBookID.String())
	assert.Nil(t, res[0].LinkedFeedItemID)
	assert.Equal(t, 0, res[0].SortOrder)
}

func TestLearningPath_MapsLinkedState(t *testing.T) {
	bookID, feedID := uuid.New(), uuid.New()
	lp := &models.LearningPath{
		ID: uuid.New(),
		Modules: []models.Module{{Items: []models.Item{{
			LinkedBookID: &bookID,
			LinkedBook:   &models.LinkedBook{Title: "B", ProgressPercent: 40},
		}}}},
		Resources: []models.Resource{{
			LinkedFeedItemID: &feedID,
			LinkedFeedItem:   &models.LinkedFeedItem{Title: "F", Read: true},
		}},
	}

	out := LearningPath(lp)
	item := out.Modules[0].Items[0]
	assert.Equal(t, bookID.String(), item.GetLinkedBookId())
	assert.Equal(t, int32(40), item.LinkedBook.ProgressPercent)
	res := out.Resources[0]
	assert.Equal(t, feedID.String(), res.GetLinkedFeedItemId())
	assert.True(t, res.LinkedFeedItem.Read)
	assert.Nil(t, LearningPath(nil))
}

func TestProgress_MapsCounts(t *testing.T) {
	id, mid := uuid.New(), uuid.New()
	out := Progress(&models.Progress{
		PathID: id, Title: "T", TotalItems: 3, CompletedItems: 1,
		Modules: []models.ModuleProgress{
			{ID: mid, Title: "M", TotalItems: 3, CompletedItems: 1},
		},
	})
	assert.Equal(t, id.String(), out.LearningPathId)
	assert.Equal(t, int32(3), out.TotalItems)
	require.Len(t, out.Modules, 1)
	assert.Equal(t, mid.String(), out.Modules[0].Id)
	assert.Equal(t, int32(1), out.Modules[0].CompletedItems)
}

func TestTodoistSyncState_MapsItems(t *testing.T) {
	task := "task-1"
	state := &services.SyncState{
		ProjectID:    "proj",
		ActiveModule: "Basics",
		Paused:       true,
		Items: []models.Item{
			{ID: uuid.New(), Description: "a", TodoistTaskID: &task},
			{ID: uuid.New(), Description: "b"},
		},
	}
	state.Status.Connected = true

	out := TodoistSyncState(state)
	assert.True(t, out.Connected)
	assert.True(t, out.Paused)
	assert.Equal(t, "proj", out.TodoistProjectId)
	require.Len(t, out.Items, 2)
	assert.Equal(t, "task-1", out.Items[0].TodoistTaskId)
	assert.Empty(t, out.Items[1].TodoistTaskId)
}

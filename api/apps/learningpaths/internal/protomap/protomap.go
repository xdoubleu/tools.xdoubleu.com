// Package protomap converts learningpaths domain models to and from the
// generated wire messages.
package protomap

import (
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

// LearningPath maps a path with its tree to the wire message.
func LearningPath(lp *models.LearningPath) *learningpathsv1.LearningPath {
	if lp == nil {
		return nil
	}
	modules := make([]*learningpathsv1.Module, len(lp.Modules))
	for i, m := range lp.Modules {
		modules[i] = protoModule(&m)
	}
	resources := make([]*learningpathsv1.Resource, len(lp.Resources))
	for i, r := range lp.Resources {
		resources[i] = protoResource(&r)
	}
	return &learningpathsv1.LearningPath{
		Id:                 lp.ID.String(),
		UserId:             lp.UserID,
		Title:              lp.Title,
		Goal:               lp.Goal,
		Routine:            lp.Routine,
		CreatedAt:          lp.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          lp.UpdatedAt.Format(time.RFC3339),
		Modules:            modules,
		Resources:          resources,
		ReminderSchedules:  lp.ReminderSchedules,
		Paused:             lp.Paused,
		ModuleCount:        lp.ModuleCount,
		ItemCount:          lp.ItemCount,
		CompletedItemCount: lp.CompletedItemCount,
	}
}

// LearningPaths maps a list of paths to wire messages.
func LearningPaths(list []models.LearningPath) []*learningpathsv1.LearningPath {
	result := make([]*learningpathsv1.LearningPath, len(list))
	for i := range list {
		result[i] = LearningPath(&list[i])
	}
	return result
}

func protoModule(m *models.Module) *learningpathsv1.Module {
	if m == nil {
		return nil
	}
	items := make([]*learningpathsv1.Item, len(m.Items))
	for i, it := range m.Items {
		items[i] = protoItem(&it)
	}
	sortOrder := int32(m.SortOrder) //nolint:gosec // int32 safe for domain values
	return &learningpathsv1.Module{
		Id:             m.ID.String(),
		LearningPathId: m.LearningPathID.String(),
		Title:          m.Title,
		SortOrder:      sortOrder,
		Items:          items,
		Quiz:           protoQuiz(m.Quiz),
	}
}

func protoQuiz(quiz []models.QuizQuestion) []*learningpathsv1.QuizQuestion {
	result := make([]*learningpathsv1.QuizQuestion, len(quiz))
	for i, q := range quiz {
		result[i] = &learningpathsv1.QuizQuestion{
			Prompt:             q.Prompt,
			Options:            q.Options,
			CorrectAnswerIndex: quizCorrectIndex(q),
		}
	}
	return result
}

//nolint:gosec // int32 safe for domain values
func quizCorrectIndex(q models.QuizQuestion) int32 {
	return int32(q.CorrectAnswerIndex)
}

// protoItem maps one item to the wire message.
func protoItem(it *models.Item) *learningpathsv1.Item {
	if it == nil {
		return nil
	}

	out := &learningpathsv1.Item{
		Id:          it.ID.String(),
		ModuleId:    it.ModuleID.String(),
		Type:        it.Type,
		Description: it.Description,
		SortOrder:   int32(it.SortOrder), //nolint:gosec // int32 safe for domain values
		Completed:   it.Completed,
		Due:         it.Due,
	}
	if it.LinkedBookID != nil {
		id := it.LinkedBookID.String()
		out.LinkedBookId = &id
	}
	if it.LinkedBook != nil {
		out.LinkedBook = protoLinkedBook(it.LinkedBook)
	}
	return out
}

func protoResource(r *models.Resource) *learningpathsv1.Resource {
	if r == nil {
		return nil
	}
	sortOrder := int32(r.SortOrder) //nolint:gosec // int32 safe for domain values

	var linkedBookID, linkedFeedItemID *string
	if r.LinkedBookID != nil {
		id := r.LinkedBookID.String()
		linkedBookID = &id
	}
	if r.LinkedFeedItemID != nil {
		id := r.LinkedFeedItemID.String()
		linkedFeedItemID = &id
	}

	return &learningpathsv1.Resource{
		Id:               r.ID.String(),
		LearningPathId:   r.LearningPathID.String(),
		Text:             r.Text,
		SortOrder:        sortOrder,
		LinkedBookId:     linkedBookID,
		LinkedFeedItemId: linkedFeedItemID,
		LinkedBook:       protoLinkedBook(r.LinkedBook),
		LinkedFeedItem:   protoLinkedFeedItem(r.LinkedFeedItem),
	}
}

func protoLinkedBook(lb *models.LinkedBook) *learningpathsv1.LinkedBook {
	if lb == nil {
		return nil
	}
	//nolint:gosec // safe for domain values
	progressPercent := int32(lb.ProgressPercent)
	return &learningpathsv1.LinkedBook{
		Title:           lb.Title,
		Status:          lb.Status,
		ProgressPercent: progressPercent,
		CoverUrl:        lb.CoverURL,
	}
}

func protoLinkedFeedItem(li *models.LinkedFeedItem) *learningpathsv1.LinkedFeedItem {
	if li == nil {
		return nil
	}
	return &learningpathsv1.LinkedFeedItem{
		Title:      li.Title,
		SourceUrl:  li.SourceURL,
		Read:       li.Read,
		Bookmarked: li.Bookmarked,
	}
}

// ModulesFromProto converts Module messages to models. Wire IDs are ignored:
// Create/Update replace the whole tree.
func ModulesFromProto(in []*learningpathsv1.Module) []models.Module {
	modules := make([]models.Module, len(in))
	for i, m := range in {
		if m == nil {
			continue
		}
		items := make([]models.Item, len(m.Items))
		for j, it := range m.Items {
			if it == nil {
				continue
			}
			//nolint:exhaustruct //ID/ModuleID assigned by the repository
			items[j] = models.Item{
				Type:         it.Type,
				Description:  it.Description,
				SortOrder:    j,
				Completed:    it.Completed,
				Due:          it.Due,
				LinkedBookID: parseOptionalUUID(it.LinkedBookId),
				ExternalBook: externalBookFromProto(it.ExternalBook),
			}
		}
		//nolint:exhaustruct //ID/LearningPathID assigned by the repository
		modules[i] = models.Module{
			Title:     m.Title,
			SortOrder: i,
			Items:     items,
			Quiz:      quizFromProto(m.Quiz),
		}
	}
	return modules
}

func externalBookFromProto(
	in *learningpathsv1.ExternalBookRef,
) *models.ExternalBookRef {
	if in == nil || in.Provider == "" || in.ProviderId == "" {
		return nil
	}
	return &models.ExternalBookRef{Provider: in.Provider, ProviderID: in.ProviderId}
}

func quizFromProto(in []*learningpathsv1.QuizQuestion) []models.QuizQuestion {
	quiz := make([]models.QuizQuestion, len(in))
	for i, q := range in {
		if q == nil {
			continue
		}
		quiz[i] = models.QuizQuestion{
			Prompt:             q.Prompt,
			Options:            q.Options,
			CorrectAnswerIndex: int(q.CorrectAnswerIndex),
		}
	}
	return quiz
}

// Progress maps a path's completion counts to the wire response.
func Progress(p *models.Progress) *learningpathsv1.GetLearningPathProgressResponse {
	modules := make([]*learningpathsv1.ModuleProgress, len(p.Modules))
	for i, m := range p.Modules {
		modules[i] = &learningpathsv1.ModuleProgress{
			Id:             m.ID.String(),
			Title:          m.Title,
			TotalItems:     m.TotalItems,
			CompletedItems: m.CompletedItems,
		}
	}
	return &learningpathsv1.GetLearningPathProgressResponse{
		LearningPathId: p.PathID.String(),
		Title:          p.Title,
		TotalItems:     p.TotalItems,
		CompletedItems: p.CompletedItems,
		Modules:        modules,
	}
}

// ResourcesFromProto converts Resource messages to models. A non-UUID link ID is
// treated as absent; validateResourceLinks catches well-formed bad IDs.
func ResourcesFromProto(in []*learningpathsv1.Resource) []models.Resource {
	resources := make([]models.Resource, len(in))
	for i, r := range in {
		if r == nil {
			continue
		}
		//nolint:exhaustruct //ID/LearningPathID assigned by the repository
		resources[i] = models.Resource{
			Text:             r.Text,
			SortOrder:        i,
			LinkedBookID:     parseOptionalUUID(r.LinkedBookId),
			LinkedFeedItemID: parseOptionalUUID(r.LinkedFeedItemId),
		}
	}
	return resources
}

func parseOptionalUUID(s *string) *uuid.UUID {
	if s == nil || *s == "" {
		return nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil
	}
	return &id
}

// TodoistSyncState maps a path's stored Todoist state to the wire response.
func TodoistSyncState(
	state *services.SyncState,
) *learningpathsv1.GetTodoistSyncStateResponse {
	items := make([]*learningpathsv1.TodoistItemState, len(state.Items))
	for i, it := range state.Items {
		items[i] = &learningpathsv1.TodoistItemState{
			ItemId:        it.ID.String(),
			Description:   it.Description,
			Due:           it.Due,
			Completed:     it.Completed,
			TodoistTaskId: derefString(it.TodoistTaskID),
		}
	}
	return &learningpathsv1.GetTodoistSyncStateResponse{
		Connected:        state.Status.Connected,
		NeedsReconnect:   state.Status.NeedsReconnect,
		RequestedScope:   state.Status.RequestedScope,
		TodoistProjectId: state.ProjectID,
		ActiveModule:     state.ActiveModule,
		Items:            items,
		Paused:           state.Paused,
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

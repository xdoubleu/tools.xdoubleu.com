package learningpaths

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type learningPathsConnectHandler struct {
	app *LearningPaths
}

//nolint:lll //type names are what they are
var _ learningpathsv1connect.LearningPathsServiceHandler = (*learningPathsConnectHandler)(nil)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		ctx,
		constants.UserContextKey,
	)
}

func mapError(err error) error {
	return connecttools.MapError(err)
}

func protoLearningPath(lp *models.LearningPath) *learningpathsv1.LearningPath {
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
		Id:        lp.ID.String(),
		UserId:    lp.UserID,
		Title:     lp.Title,
		Goal:      lp.Goal,
		Routine:   lp.Routine,
		CreatedAt: lp.CreatedAt.Format(time.RFC3339),
		UpdatedAt: lp.UpdatedAt.Format(time.RFC3339),
		Modules:   modules,
		Resources: resources,
	}
}

func protoLearningPaths(list []models.LearningPath) []*learningpathsv1.LearningPath {
	result := make([]*learningpathsv1.LearningPath, len(list))
	for i := range list {
		result[i] = protoLearningPath(&list[i])
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
	}
}

func protoItem(it *models.Item) *learningpathsv1.Item {
	if it == nil {
		return nil
	}
	return &learningpathsv1.Item{
		Id:          it.ID.String(),
		ModuleId:    it.ModuleID.String(),
		Type:        it.Type,
		Description: it.Description,
		SortOrder:   int32(it.SortOrder), //nolint:gosec // int32 safe for domain values
		Completed:   it.Completed,
	}
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
	return &learningpathsv1.LinkedBook{
		Title:           lb.Title,
		Status:          lb.Status,
		ProgressPercent: int32(lb.ProgressPercent), //nolint:gosec // safe for domain values
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

// dtoToModules converts request-level Module messages to domain models. IDs
// on the wire are ignored — Create/Update wholesale-replace the tree (see
// LearningPathsRepository.ReplaceModules), so the client never needs to
// address an existing module/item by ID.
func dtoToModules(in []*learningpathsv1.Module) []models.Module {
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
				Type:        it.Type,
				Description: it.Description,
				SortOrder:   j,
				Completed:   it.Completed,
			}
		}
		//nolint:exhaustruct //ID/LearningPathID assigned by the repository
		modules[i] = models.Module{
			Title:     m.Title,
			SortOrder: i,
			Items:     items,
		}
	}
	return modules
}

// progressFromLearningPath derives per-module and overall completion counts
// from a fully-populated learning path. Kept alongside the other
// model-to-proto conversions rather than in the MCP layer, since it's shaped
// by GetLearningPathProgress's proto response and has nothing MCP-specific
// about it — the same computation would back a future UI progress bar too.
func progressFromLearningPath(
	lp *models.LearningPath,
) *learningpathsv1.GetLearningPathProgressResponse {
	modules := make([]*learningpathsv1.ModuleProgress, len(lp.Modules))
	var totalItems, completedItems int32
	for i, m := range lp.Modules {
		var moduleTotal, moduleCompleted int32
		for _, it := range m.Items {
			moduleTotal++
			if it.Completed {
				moduleCompleted++
			}
		}
		modules[i] = &learningpathsv1.ModuleProgress{
			Id:             m.ID.String(),
			Title:          m.Title,
			TotalItems:     moduleTotal,
			CompletedItems: moduleCompleted,
		}
		totalItems += moduleTotal
		completedItems += moduleCompleted
	}
	return &learningpathsv1.GetLearningPathProgressResponse{
		LearningPathId: lp.ID.String(),
		Title:          lp.Title,
		TotalItems:     totalItems,
		CompletedItems: completedItems,
		Modules:        modules,
	}
}

// dtoToResources converts request-level Resource messages to domain models.
// An unparseable linked_book_id/linked_feed_item_id (not a valid UUID) is
// silently treated as absent rather than rejected here — a well-formed but
// non-existent/foreign-owned ID is still caught by the service layer's
// validateResourceLinks call.
func dtoToResources(in []*learningpathsv1.Resource) []models.Resource {
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

package services

import (
	"context"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// JourneyTopics is the push channel a watched journey is published on.
type JourneyTopics interface {
	// EnsureTopic registers a topic for journeyID if missing.
	EnsureTopic(journeyID string)
}

// JourneyWatchService fetches a journey's live state and opens its push topic
// so the client can subscribe right after.
type JourneyWatchService struct {
	detail *JourneyDetailService
	topics JourneyTopics
}

func NewJourneyWatchService(
	detail *JourneyDetailService, topics JourneyTopics,
) *JourneyWatchService {
	return &JourneyWatchService{detail: detail, topics: topics}
}

// Watch returns journeyID's current detail and ensures its topic exists. An
// invalid or unresolvable journey opens no topic.
func (s *JourneyWatchService) Watch(
	ctx context.Context, journeyID string,
) (*models.JourneyDetail, error) {
	if journeyID == "" {
		return nil, ErrJourneyIDRequired
	}
	detail, err := s.detail.GetJourneyDetail(ctx, journeyID)
	if err != nil {
		return nil, err
	}
	s.topics.EnsureTopic(journeyID)
	return detail, nil
}

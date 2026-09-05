package services

import (
	"context"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
)

// FeedInfoService answers GetFeedInfo — just enough of the stored feed
// metadata to drive the required CC BY attribution string on /trains.
type FeedInfoService struct {
	repos *repositories.Repositories
}

func NewFeedInfoService(repos *repositories.Repositories) *FeedInfoService {
	return &FeedInfoService{repos: repos}
}

// FeedVersion returns the currently imported feed's feed_version, or "" if
// nothing has been imported yet.
func (s *FeedInfoService) FeedVersion(ctx context.Context) (string, error) {
	info, err := s.repos.Feed.GetFeedInfo(ctx)
	if err != nil {
		return "", err
	}
	if info == nil {
		return "", nil
	}
	return info.FeedVersion, nil
}

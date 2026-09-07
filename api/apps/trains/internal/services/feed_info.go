package services

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
)

// FeedInfoService answers GetFeedInfo — just enough of the stored feed
// metadata to drive the required CC BY attribution string on /trains, plus
// the import timestamp that says whether that timetable is still current.
type FeedInfoService struct {
	repos *repositories.Repositories
}

func NewFeedInfoService(repos *repositories.Repositories) *FeedInfoService {
	return &FeedInfoService{repos: repos}
}

// FeedInfo is the stored feed's version and the time the import that
// produced it ran. Both are zero when nothing has been imported yet.
type FeedInfo struct {
	FeedVersion string
	ImportedAt  *time.Time
}

// FeedInfo returns the currently imported feed's metadata, or a zero value
// if nothing has been imported yet.
func (s *FeedInfoService) FeedInfo(ctx context.Context) (FeedInfo, error) {
	info, err := s.repos.Feed.GetFeedInfo(ctx)
	if err != nil {
		return FeedInfo{}, err
	}
	if info == nil {
		//nolint:exhaustruct //nothing imported yet
		return FeedInfo{}, nil
	}
	return FeedInfo{
		FeedVersion: info.FeedVersion,
		ImportedAt:  info.ImportedAt,
	}, nil
}

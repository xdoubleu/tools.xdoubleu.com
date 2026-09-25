package services

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/repositories"
)

// FeedInfoService answers GetFeedInfo: feed version for the CC BY
// attribution, import time, and translation coverage.
type FeedInfoService struct {
	repos *repositories.Repositories
}

func NewFeedInfoService(repos *repositories.Repositories) *FeedInfoService {
	return &FeedInfoService{repos: repos}
}

// FeedInfo is zero when nothing has been imported.
type FeedInfo struct {
	FeedVersion  string
	ImportedAt   *time.Time
	Translations models.TranslationCoverage
}

// FeedInfo returns the imported feed's metadata, or zero if none.
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
		FeedVersion:  info.FeedVersion,
		ImportedAt:   info.ImportedAt,
		Translations: info.Translations,
	}, nil
}

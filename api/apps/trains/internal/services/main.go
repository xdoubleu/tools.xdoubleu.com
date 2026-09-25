// Package services holds the trains app's business logic.
package services

import (
	"context"
	"log/slog"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
)

type Services struct {
	StaticImport  *StaticImportService
	Journey       *JourneyService
	Stations      *StationsService
	FeedInfo      *FeedInfoService
	Realtime      *RealtimeService
	JourneyDetail *JourneyDetailService
	JourneyWS     *JourneyWSService
	SavedCommutes *SavedCommutesService
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	repos *repositories.Repositories,
	bmcClient bmc.Client,
	allowedOrigins []string,
) *Services {
	realtime := NewRealtimeService(logger, bmcClient, repos.Feed)
	journey := NewJourneyService(logger, repos)
	detail := NewJourneyDetailService(repos, realtime, journey)
	journeyWS := NewJourneyWSService(ctx, logger, allowedOrigins, detail)
	realtime.OnUpdate(func() { journeyWS.PushAll(ctx) })

	return &Services{
		StaticImport:  NewStaticImportService(logger, repos, bmcClient),
		Journey:       journey,
		Stations:      NewStationsService(repos),
		FeedInfo:      NewFeedInfoService(repos),
		Realtime:      realtime,
		JourneyDetail: detail,
		JourneyWS:     journeyWS,
		SavedCommutes: NewSavedCommutesService(repos),
	}
}

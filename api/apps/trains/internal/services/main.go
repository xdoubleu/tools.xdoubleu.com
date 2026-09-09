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
	detail := NewJourneyDetailService(repos, realtime)
	journeyWS := NewJourneyWSService(ctx, logger, allowedOrigins, detail)
	// A journey page stays open and pushed-to for the length of a trip
	// (issue #1394): every realtime poll cycle rebroadcasts fresh detail to
	// each subscribed journey's topic rather than waiting for the client to
	// re-ask.
	realtime.OnUpdate(func() { journeyWS.PushAll(ctx) })

	return &Services{
		StaticImport:  NewStaticImportService(logger, repos, bmcClient),
		Journey:       NewJourneyService(logger, repos),
		Stations:      NewStationsService(repos),
		FeedInfo:      NewFeedInfoService(repos),
		Realtime:      realtime,
		JourneyDetail: detail,
		JourneyWS:     journeyWS,
		SavedCommutes: NewSavedCommutesService(repos),
	}
}

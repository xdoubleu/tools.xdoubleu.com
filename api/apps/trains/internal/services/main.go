// Package services holds the trains app's business logic.
package services

import (
	"context"
	"log/slog"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

type Services struct {
	Journey       *JourneyService
	Stations      *StationsService
	JourneyDetail *JourneyDetailService
	JourneyWS     *JourneyWSService
	SavedCommutes *SavedCommutesService
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	repos *repositories.Repositories,
	client irail.Client,
	allowedOrigins []string,
) *Services {
	stations := NewStationsService(client)
	journey := NewJourneyService(client, stations)
	detail := NewJourneyDetailService(client, stations, journey)

	return &Services{
		Journey:       journey,
		Stations:      stations,
		JourneyDetail: detail,
		JourneyWS:     NewJourneyWSService(ctx, logger, allowedOrigins, detail),
		SavedCommutes: NewSavedCommutesService(repos, stations),
	}
}

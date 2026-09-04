// Package services holds the trains app's business logic — for this slice
// (issue #1390) just the daily GTFS static feed import.
package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
)

type Services struct {
	StaticImport *StaticImportService
	Journey      *JourneyService
}

func New(
	logger *slog.Logger,
	repos *repositories.Repositories,
	bmcClient bmc.Client,
) *Services {
	return &Services{
		StaticImport: NewStaticImportService(logger, repos, bmcClient),
		Journey:      NewJourneyService(logger, repos),
	}
}

package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/safedial"
)

type Services struct {
	Auth     auth.Service
	Calendar *CalendarService
}

// New wires the app's services. allowPrivate lets the calendar fetcher reach
// private/loopback addresses — true outside production only, since tests and
// local development point at httptest servers.
func New(
	logger *slog.Logger,
	repos *repositories.Repositories,
	auth auth.Service,
	allowPrivate bool,
) *Services {
	return &Services{
		Auth: auth,
		Calendar: &CalendarService{
			logger: logger,
			repo:   repos.Calendar,
			client: safedial.Client(
				calendarFetchTimeout,
				maxCalendarRedirects,
				allowPrivate,
			),
		},
	}
}

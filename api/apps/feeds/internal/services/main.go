package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/feeds/internal/repositories"
	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/notifications"
	globalrepositories "tools.xdoubleu.com/internal/repositories"
)

type Services struct {
	Auth  auth.Service
	Feeds *FeedService
}

func New(
	logger *slog.Logger,
	repos *repositories.Repositories,
	webFetchClient webfetch.Client,
	inboundDomain string,
	authService auth.Service,
	notifications *notifications.Service,
	appUsersRepo *globalrepositories.AppUsersRepository,
	webURL string,
) *Services {
	return &Services{
		Auth: authService,
		Feeds: NewFeedService(
			logger,
			repos.Feeds,
			repos.Items,
			webFetchClient,
			inboundDomain,
			notifications,
			appUsersRepo,
			webURL,
		),
	}
}

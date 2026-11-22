package services

import (
	"log/slog"

	"tools.xdoubleu.com/internal/kit/threading"
	"tools.xdoubleu.com/internal/kit/webfetch"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/notifications"
)

type Services struct {
	Auth  auth.Service
	Feeds *FeedService
}

// Stores are the storage ports the services run on.
type Stores struct {
	Feeds       feedsStore
	Items       itemsStore
	FilterRules filterRulesStore
}

func New(
	logger *slog.Logger,
	background *threading.Background,
	stores Stores,
	webFetchClient webfetch.Client,
	inboundDomain string,
	authService auth.Service,
	notifications *notifications.Service,
	users userLookup,
	webURL string,
	mail ReceivedMail,
) *Services {
	return &Services{
		Auth: authService,
		Feeds: NewFeedService(
			logger,
			background,
			stores.Feeds,
			stores.Items,
			stores.FilterRules,
			webFetchClient,
			inboundDomain,
			notifications,
			users,
			webURL,
			mail,
		),
	}
}

package services

import (
	"context"
	"log/slog"

	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/pkg/googlebooks"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/apps/backlog/pkg/unicat"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/progressws"
)

type Services struct {
	Auth         auth.Service
	Steam        *SteamService
	Books        *BookService
	Conversion   *ConversionService
	Progress     *ProgressService
	Integrations *IntegrationsService
	Kobo         *KoboService
	WebSocket    *progressws.Service
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	jobQueue *threading.JobQueue,
	repositories *repositories.Repositories,
	steamFactory func(apiKey string) steam.Client,
	external openlibrary.Client,
	googleBooks googlebooks.Client,
	uniCat unicat.Client,
	objectStore objectstore.Client,
	authService auth.Service,
) *Services {
	integrations := &IntegrationsService{
		repo: repositories.Integrations,
	}
	kobo := &KoboService{
		repo: repositories.KoboDevices,
	}

	booksSvc := &BookService{
		logger:       logger,
		books:        repositories.Books,
		bookFiles:    repositories.BookFiles,
		objectStore:  objectStore,
		readingState: repositories.ReadingState,
		external:     external,
		googleBooks:  googleBooks,
		uniCat:       uniCat,
		booksResync:  nil, // nil → resyncRepo() falls back to books
	}
	steamSvc := &SteamService{
		logger:        logger,
		clientFactory: steamFactory,
		steamAPIKey:   config.SteamAPIKey,
		steam:         repositories.Steam,
		progress:      repositories.Progress,
		integrations:  integrations,
	}
	progressSvc := NewProgressService(repositories.Progress, steamSvc)

	conversionSvc := NewConversionService(
		logger,
		repositories.BookFiles,
		objectStore,
		nil, // converter: defaults to kepubify
		nil, // convertPDF: defaults to calibrePDFConverter (ebook-convert subprocess)
	)

	return &Services{
		Auth:         authService,
		Steam:        steamSvc,
		Books:        booksSvc,
		Conversion:   conversionSvc,
		Progress:     progressSvc,
		Integrations: integrations,
		Kobo:         kobo,
		WebSocket: progressws.NewService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}

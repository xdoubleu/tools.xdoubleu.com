package services

import (
	"context"
	"log/slog"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/apps/books/pkg/webfetch"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/progressws"
)

type Services struct {
	Auth       auth.Service
	Books      *BookService
	Conversion *ConversionService
	Positions  *PositionService
	Progress   *ProgressService
	Kobo       *KoboService
	KoboLog    *KoboLogStore
	Ingest     *IngestService
	WebSocket  *progressws.Service
	WebPub     *WebPubService
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	jobQueue *jobqueue.JobQueue,
	repositories *repositories.Repositories,
	uniCat unicat.Client,
	hardcoverClient hardcover.Client,
	objectStore objectstore.Client,
	webFetchClient webfetch.Client,
	authService auth.Service,
) *Services {
	kobo := &KoboService{
		repo: repositories.KoboDevices,
	}

	koboLog := NewKoboLogStore()

	booksSvc := &BookService{
		logger:       logger,
		books:        repositories.Books,
		bookFiles:    repositories.BookFiles,
		objectStore:  objectStore,
		readingState: repositories.ReadingState,
		uniCat:       uniCat,
		hardcover:    hardcoverClient,
		resyncSource: repositories.Books,
		coverClient:  newCoverClient(config.Env),
		seriesCache:  newSeriesCache(),
	}

	conversionSvc := NewConversionService(
		logger,
		repositories.Books,
		repositories.BookFiles,
		objectStore,
		nil, // converter: defaults to kepubify
		nil, // convertPDF: defaults to goPDFConverter (pure-Go, go-pdfium)
	)

	ingestSvc := NewIngestService(
		logger,
		repositories,
		webFetchClient,
	)

	return &Services{
		Auth:       authService,
		Books:      booksSvc,
		Conversion: conversionSvc,
		Positions: newPositionService(
			logger, repositories.BookFiles, objectStore, translateBudget,
		),
		Progress: NewProgressService(repositories.Progress),
		Kobo:     kobo,
		KoboLog:  koboLog,
		Ingest:   ingestSvc,
		WebPub:   newWebPubService(booksSvc),
		WebSocket: progressws.NewService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}

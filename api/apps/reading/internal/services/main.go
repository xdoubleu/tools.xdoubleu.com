package services

import (
	"context"
	"log/slog"

	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/reading/internal/repositories"
	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
	"tools.xdoubleu.com/apps/reading/pkg/hardcover"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	"tools.xdoubleu.com/apps/reading/pkg/unicat"
	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/progressws"
)

type Services struct {
	Auth       auth.Service
	Books      *BookService
	Conversion *ConversionService
	Progress   *ProgressService
	Kobo       *KoboService
	KoboLog    *KoboLogStore
	Ingest     *IngestService
	Feeds      *FeedService
	WebSocket  *progressws.Service
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	jobQueue *threading.JobQueue,
	repositories *repositories.Repositories,
	uniCat unicat.Client,
	hardcoverClient hardcover.Client,
	objectStore objectstore.Client,
	webFetchClient webfetch.Client,
	arxivClient arxiv.Client,
	htmlConvert HTMLConverter,
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
		booksResync:  nil, // nil → resyncRepo() falls back to books
	}

	conversionSvc := NewConversionService(
		logger,
		repositories.BookFiles,
		objectStore,
		nil, // converter: defaults to kepubify
		nil, // convertPDF: defaults to calibrePDFConverter (ebook-convert subprocess)
	)

	ingestSvc := NewIngestService(
		logger,
		booksSvc,
		repositories,
		objectStore,
		webFetchClient,
		arxivClient,
		htmlConvert, // nil defaults to calibreHTMLConverter
	)

	feedsSvc := NewFeedService(
		logger,
		repositories.Feeds,
		ingestSvc,
		booksSvc,
		conversionSvc,
		webFetchClient,
	)

	return &Services{
		Auth:       authService,
		Books:      booksSvc,
		Conversion: conversionSvc,
		Progress:   NewProgressService(repositories.Progress),
		Kobo:       kobo,
		KoboLog:    koboLog,
		Ingest:     ingestSvc,
		Feeds:      feedsSvc,
		WebSocket: progressws.NewService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}

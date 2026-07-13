package services

import (
	"context"
	"log/slog"

	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
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
	WebSocket  *progressws.Service
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	jobQueue *threading.JobQueue,
	repositories *repositories.Repositories,
	external openlibrary.Client,
	uniCat unicat.Client,
	hardcoverClient hardcover.Client,
	objectStore objectstore.Client,
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
		external:     external,
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

	return &Services{
		Auth:       authService,
		Books:      booksSvc,
		Conversion: conversionSvc,
		Progress:   NewProgressService(repositories.Progress),
		Kobo:       kobo,
		KoboLog:    koboLog,
		WebSocket: progressws.NewService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}

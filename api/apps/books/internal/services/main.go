package services

import (
	"context"
	"log/slog"

	"golang.org/x/sync/singleflight"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/internal/services/conversion"
	"tools.xdoubleu.com/apps/books/internal/services/kobo"
	"tools.xdoubleu.com/apps/books/internal/services/positions"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/progressws"
	"tools.xdoubleu.com/internal/threading"
	"tools.xdoubleu.com/internal/webfetch"
)

type Services struct {
	Auth       auth.Service
	Books      *BookService
	Conversion *conversion.Service
	KEPUBQueue *conversion.KEPUBQueue
	Figures    *conversion.FigureService
	Positions  *positions.PositionService
	Physical   *positions.PhysicalService
	Progress   *ProgressService
	Kobo       *kobo.Service
	KoboLog    *kobo.LogStore
	Ingest     *IngestService
	WebSocket  *progressws.Service
	WebPub     *WebPubService
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	background *threading.Background,
	config config.Config,
	jobQueue *jobqueue.JobQueue,
	repositories *repositories.Repositories,
	uniCat unicat.Client,
	hardcoverClient hardcover.Client,
	objectStore objectstore.Client,
	webFetchClient webfetch.Client,
	authService auth.Service,
) *Services {
	koboSvc := kobo.NewService(repositories.KoboDevices)

	koboLog := kobo.NewLogStore()

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
		coverWarms:   singleflight.Group{},
		seriesCache:  newSeriesCache(),
		physical:     nil, // set once the position service exists
	}

	conversionSvc := conversion.NewService(
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

	positionSvc := positions.NewPositionService(
		logger, repositories.BookFiles, objectStore, positions.TranslateBudget,
	)
	booksSvc.physical = positions.NewPhysicalService(
		logger, background, repositories.Books, repositories.ReadingState, positionSvc,
	)

	return &Services{
		Auth:       authService,
		Books:      booksSvc,
		Conversion: conversionSvc,
		KEPUBQueue: conversion.NewKEPUBQueue(
			logger, repositories.KEPUBRequests, conversionSvc,
		),
		Figures:   conversion.NewFigureService(logger, repositories.BookFiles, objectStore),
		Positions: positionSvc,
		Physical:  booksSvc.physical,
		Progress:  NewProgressService(repositories.Progress),
		Kobo:      koboSvc,
		KoboLog:   koboLog,
		Ingest:    ingestSvc,
		WebPub:    newWebPubService(booksSvc),
		WebSocket: progressws.NewService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}

package kobo

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// Library is the book/reading-state storage the protocol reads and writes.
type Library interface {
	ListKoboSyncBooks(ctx context.Context, userID string) ([]models.KoboSyncBook, error)
	GetKoboSyncBook(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (models.KoboSyncBook, error)
	ListKoboRemovals(ctx context.Context, userID string) ([]models.KoboRemoval, error)
	UpdateKoboLastSynced(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		converterVersion int16,
		coverImageID string,
	) error
	WarmUserCovers(ctx context.Context, userID string, bookIDs []uuid.UUID)
	GetUserBook(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (*models.UserBook, error)

	ListReadingStates(
		ctx context.Context, userID string,
	) (map[uuid.UUID]*models.BookReadingState, error)
	GetReadingState(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (*models.BookReadingState, error)
	ListKoboDeviceHeld(
		ctx context.Context, deviceID string,
	) (map[uuid.UUID]time.Time, error)
	SyncKoboDeviceReadingStates(
		ctx context.Context,
		deviceID string,
		bookIDs []uuid.UUID,
		states map[uuid.UUID]*models.BookReadingState,
		held map[uuid.UUID]time.Time,
		pending map[uuid.UUID]bool,
	) ([]*models.BookReadingState, error)
	UpdateKoboReadingProgress(
		ctx context.Context, deviceID string, state models.BookReadingState,
	) error
	ClearKoboLocation(
		ctx context.Context, userID string, bookID uuid.UUID, loc models.KoboLocation,
	) error
	SetKoboPosition(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		loc models.KoboLocation,
		pos models.ReadingPosition,
	) error

	LinkKoboStoreBooks(
		ctx context.Context, userID string, storeBooks []models.KoboStoreBook,
	) error
	RecordKoboStoreReading(
		ctx context.Context, userID string, reading models.KoboStoreReading,
	) (models.KoboStoreOutcome, error)
}

// Positions translates between Kobo bookmarks and neutral reading positions.
type Positions interface {
	KoboLocations(
		ctx context.Context,
		userID string,
		states map[uuid.UUID]*models.BookReadingState,
		replaced map[uuid.UUID]bool,
	) (map[uuid.UUID]*models.KoboLocation, map[uuid.UUID]bool)
	KoboPosition(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		loc models.KoboLocation,
		outdated bool,
	) (*models.ReadingPosition, bool)
	BackfillKoboPosition(
		ctx context.Context,
		userID string,
		bookID uuid.UUID,
		loc models.KoboLocation,
		store func(context.Context, models.ReadingPosition) error,
	)
}

// KEPUBs reports and repairs stale KEPUB conversions.
type KEPUBs interface {
	IsKEPUBStale(version int16) bool
	Request(ctx context.Context, userID string, bookID uuid.UUID) error
}

// Background runs request-detached work.
type Background interface {
	Go(ctx context.Context, name string, fn func(context.Context))
}

// Deps are the collaborators a Protocol is built from.
type Deps struct {
	Library    Library
	Positions  Positions
	KEPUBs     KEPUBs
	Background Background
	Logger     *slog.Logger
	// StoreBaseURL is the upstream Kobo store the protocol proxies to.
	StoreBaseURL string
	// PublicAPIBaseURL, when set, replaces the request origin in URLs handed
	// to the device (a reverse proxy may strip /api from the request path).
	PublicAPIBaseURL string
}

// Protocol is the Kobo device-sync protocol: payload building, the upstream
// store merge and proxy, and reading-state sync. The HTTP layer only
// authenticates, parses requests and writes responses.
type Protocol struct {
	d Deps
}

// NewProtocol builds a Protocol over d.
func NewProtocol(d Deps) *Protocol {
	return &Protocol{d: d}
}

// baseURL prefixes path with the origin the device should call back on. The
// scheme is always https (the HTTP layer enforces it).
func (p *Protocol) baseURL(path, forwardedHost, host string) string {
	if p.d.PublicAPIBaseURL != "" {
		return strings.TrimSuffix(p.d.PublicAPIBaseURL, "/") + path
	}
	if forwardedHost != "" {
		host = forwardedHost
	}
	return "https://" + host + path
}

// CoverBase derives the https://…/kobo/{token} prefix used by the cover
// image templates, by stripping the initialization path from the request.
func (p *Protocol) CoverBase(path, forwardedHost, host string) string {
	if idx := strings.Index(path, "/v1/initialization"); idx != -1 {
		path = path[:idx]
	}
	return p.baseURL(path, forwardedHost, host)
}

// LibraryBase derives the https://…/kobo/{token}/v1/library prefix.
func (p *Protocol) LibraryBase(path, forwardedHost, host string) string {
	if idx := strings.Index(path, "/v1/library"); idx != -1 {
		path = path[:idx] + "/v1/library"
	}
	return p.baseURL(path, forwardedHost, host)
}

// Init builds the /v1/initialization response: the full catalog with the keys
// we own (covers and the library endpoints) pointed at coverBase, the rest
// keeping the upstream Kobo store URLs.
func (p *Protocol) Init(coverBase string) InitResponse {
	// For init the request path is /v1/initialization (not /v1/library/...), so
	// LibraryBase can't strip it; derive the library prefix from coverBase.
	libraryBase := coverBase + "/v1/library"

	resources := catalog()
	resources["image_host"] = coverBase
	resources["image_url_quality_template"] = coverBase +
		"/{ImageId}/{width}/{height}/{Quality}/isGreyscale/image.jpg"
	resources["image_url_template"] = coverBase +
		"/{ImageId}/{width}/{height}/false/image.jpg"
	resources["library_sync"] = libraryBase + "/sync"
	resources["library_metadata"] = libraryBase + "/{Ids}/metadata"
	resources["reading_state"] = libraryBase + "/{Ids}/state"

	return InitResponse{
		Resources: resources,
		Settings: settings{
			SynchronizationDelay: 0,
			TestEmailAddress:     "",
			UserAgent:            "Kobo",
		},
		TokenList: []string{"BookEntitlement", "BookMetadata", "BookReadingState"},
	}
}

// coverRevision is appended to our books' CoverImageId. The device caches
// each image id's covers, misses included, so bumping it makes every device
// fetch all cover sizes again.
const coverRevision = "-c1"

// CoverImageID is the CoverImageId of a library book.
func CoverImageID(bookID uuid.UUID) string {
	return bookID.String() + coverRevision
}

// CoverBookID parses an image id from CoverImageID, or a bare book id from
// before the revision suffix.
func CoverBookID(imageID string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSuffix(imageID, coverRevision))
}

// CoverParams are the cover request's path values.
type CoverParams struct {
	ImageID, Width, Height, Quality, Greyscale string
}

// UpstreamCoverURL fills the store catalog's image template with the
// request's image id and dimensions.
func UpstreamCoverURL(c CoverParams) string {
	cat := catalog()
	tpl, _ := cat["image_url_template"].(string)
	if c.Quality != "" {
		tpl, _ = cat["image_url_quality_template"].(string)
	}
	greyscale := "false"
	if c.Greyscale == "true" {
		greyscale = "true"
	}
	return strings.NewReplacer(
		"{ImageId}", url.PathEscape(c.ImageID),
		"{Width}", url.PathEscape(c.Width),
		"{Height}", url.PathEscape(c.Height),
		"{Quality}", url.PathEscape(c.Quality),
		"{IsGreyscale}", greyscale,
	).Replace(tpl)
}

// isReplace reports whether b's file was regenerated since it was last
// sent, so it goes out as a ChangedEntitlement.
func isReplace(b models.KoboSyncBook) bool {
	return b.LastSyncedConverterVersion != nil &&
		*b.LastSyncedConverterVersion != b.ConverterVersion
}

// coverChanged reports whether b was synced before under a CoverImageId
// other than coverID; an unrecorded one counts as different.
func coverChanged(b models.KoboSyncBook, coverID string) bool {
	return b.LastSyncedConverterVersion != nil &&
		(b.LastSyncedCoverImageID == nil || *b.LastSyncedCoverImageID != coverID)
}

// buildMetadata builds BookMetadata; shared by sync and the metadata
// endpoint because the device cross-checks them.
func buildMetadata(b models.KoboSyncBook, libraryBase string) BookMetadata {
	id := b.BookID.String()
	return BookMetadata{
		Title:       b.Title,
		ContentType: "application/x-kobo-epub+zip",
		RevisionId:  id,
		Language:    "en",
		DownloadUrls: []downloadURL{{
			Format:   "KEPUB",
			Size:     b.Size,
			URL:      libraryBase + "/" + id + "/file",
			Platform: "Generic",
		}},
		CoverImageID: CoverImageID(b.BookID),
	}
}

// Metadata is the metadata endpoint's payload for one of our books;
// database.ErrResourceNotFound when it isn't one (the store serves it).
func (p *Protocol) Metadata(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	libraryBase string,
) ([]BookMetadata, error) {
	book, err := p.d.Library.GetKoboSyncBook(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	return []BookMetadata{buildMetadata(book, libraryBase)}, nil
}

// HasSyncBook reports whether bookID is one of userID's kobo-sync books.
func (p *Protocol) HasSyncBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (bool, error) {
	_, err := p.d.Library.GetKoboSyncBook(ctx, userID, bookID)
	return handleNotFound(err)
}

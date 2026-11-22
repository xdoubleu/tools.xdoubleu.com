// Package mcp exposes the books app's read RPCs as MCP tools.
package mcp

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/books/connect"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

const mcpAppName = "books"

type mcpProgressArgs struct {
	DateStart string `json:"date_start,omitempty" jsonschema:"window start (YYYY-MM-DD)"`
	DateEnd   string `json:"date_end,omitempty"   jsonschema:"window end (YYYY-MM-DD)"`
}

type mcpQueryArgs struct {
	Query string `json:"query" jsonschema:"search query"`
}

type mcpSeriesArgs struct {
	Name string `json:"name" jsonschema:"exact series name"`
}

type mcpExternalBookArgs struct {
	Provider   string `json:"provider"    jsonschema:"provider (unicat|hardcover)"`
	ProviderID string `json:"provider_id" jsonschema:"provider's book id"`
}

type mcpBookIDArgs struct {
	BookID string `json:"book_id" jsonschema:"catalog book id"`
}

type mcpBookSourcesArgs struct {
	BookID         string `json:"book_id"                   jsonschema:"catalog book id"`
	OverrideTitle  string `json:"override_title,omitempty"  jsonschema:"override title"`
	OverrideAuthor string `json:"override_author,omitempty" jsonschema:"override author"`
}

type mcpSourcesArgs struct {
	Sources []string `json:"sources" jsonschema:"source set e.g. [unicat]"`
}

type mcpBookFileArgs struct {
	BookID string `json:"book_id"          jsonschema:"catalog book id"`
	Format string `json:"format,omitempty" jsonschema:"format (epub|pdf|kepub)"`
}

type mcpDeviceIDArgs struct {
	ID string `json:"id" jsonschema:"Kobo device id"`
}

// tools adapts the books services to MCP tool signatures.
type tools struct {
	books        *services.BookService
	library      *services.LibraryService
	progress     *services.ProgressService
	catalog      *services.CatalogService
	coverBaseURL string
}

// Register exposes the books app's read-only RPCs on the combined apps MCP
// server. Every tool returns the calling user's own library data; the catalog
// tools need an admin.
func Register(
	srv *mcp.Server,
	svc *services.Services,
	coverBaseURL string,
) {
	t := &tools{
		books:        svc.Books,
		library:      svc.Library,
		progress:     svc.Progress,
		catalog:      svc.Catalog,
		coverBaseURL: coverBaseURL,
	}
	registerReadingLibraryTools(srv, t)
	registerReadingCatalogTools(srv, t)
}

func caller(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

func registerReadingLibraryTools(srv *mcp.Server, h *tools) {
	mcptools.AddReadTool(srv, mcpAppName, "books_get_library",
		"The user's full library: reading/wishlist/finished shelves, with each "+
			"book's formats and read suggestion (pdf for iPad/laptop when "+
			"figure-heavy, else epub for phone/Kobo).",
		h.mcpGetLibrary)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_books_progress",
		"Reading-progress graph data over a date window.", h.mcpGetBooksProgress)
	mcptools.AddReadTool(srv, mcpAppName, "books_search_library",
		"Search the user's library by text.", h.mcpSearchLibrary)
	mcptools.AddReadTool(srv, mcpAppName, "books_search_external",
		"Search external metadata providers for a book.", h.mcpSearchExternal)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_external_book",
		"Fetch one external provider's metadata for a book.",
		h.mcpGetExternalBook)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_reading_state",
		"The latest reading position for a book: percent, neutral position "+
			"(href+offset, page, or both for a PDF-sourced book), when it was "+
			"read, and for a physical copy its page and the sentence there.",
		h.mcpGetReadingState)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_series",
		"A series' volumes in order: the user's books plus Hardcover volumes "+
			"they don't have.", h.mcpGetSeries)
}

func registerReadingCatalogTools(srv *mcp.Server, h *tools) {
	mcptools.AddReadTool(srv, mcpAppName, "books_list_resync_proposals",
		"Catalog books that differ from an external source (resync wizard).",
		h.mcpListResyncProposals)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_book_sources",
		"Live-fetch one book's candidates from every configured source.",
		h.mcpGetBookSources)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_source_stats",
		"Per-source metadata coverage over the whole catalog.",
		h.mcpGetSourceStats)
	mcptools.AddReadTool(srv, mcpAppName, "books_list_books_in_exact_sources",
		"Catalog books found by exactly the given set of sources.",
		h.mcpListBooksInExactSources)
	mcptools.AddReadTool(srv, mcpAppName, "books_find_duplicates",
		"Groups of library entries judged to be the same book.",
		h.mcpFindDuplicates)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_book_file",
		"A time-limited download URL for a book's file.", h.mcpGetBookFile)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_kepub_status",
		"Whether a book has EPUB/PDF and its KEPUB conversion status.",
		h.mcpGetKEPUBStatus)
	mcptools.AddReadTool(srv, mcpAppName, "books_list_kobo_devices",
		"The user's registered Kobo devices.", h.mcpListKoboDevices)
	mcptools.AddReadTool(srv, mcpAppName, "books_get_kobo_device_logs",
		"Captured request/response logs for a Kobo device.",
		h.mcpGetKoboDeviceLogs)
	mcptools.AddReadTool(srv, mcpAppName, "books_list_kobo_store_books",
		"Kobo store (bol.com) books seen through the user's Kobo: library "+
			"match, ownership, and the last reading state mirrored from the "+
			"Kobo with its outcome.", h.mcpListKoboStoreBooks)
}

func mcpOptStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// authed returns the calling user's ID, or the unauthenticated error.
func authed(ctx context.Context) (string, error) {
	user := caller(ctx)
	if user == nil {
		return "", connect.MapError(services.ErrUnauthenticated)
	}
	return user.ID, nil
}

// authedBook is authed plus the parsed book_id argument.
func authedBook(ctx context.Context, rawBookID string) (string, uuid.UUID, error) {
	userID, err := authed(ctx)
	if err != nil {
		return "", uuid.Nil, err
	}
	bookID, err := connect.ParseBookID(rawBookID)
	if err != nil {
		return "", uuid.Nil, err
	}
	return userID, bookID, nil
}

func (h *tools) mcpGetLibrary(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := authed(ctx)
	if err != nil {
		return nil, err
	}
	data, err := h.books.LibraryView(ctx, userID)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return &booksv1.GetLibraryResponse{
		Library: connect.LibraryResponse(data, h.coverBaseURL),
	}, nil
}

func (h *tools) mcpGetBooksProgress(
	ctx context.Context, args mcpProgressArgs,
) (proto.Message, error) {
	userID, err := authed(ctx)
	if err != nil {
		return nil, err
	}
	start, end := services.ParseProgressRange(args.DateStart, args.DateEnd)
	labels, values, err := h.progress.GetByDates(ctx, userID, start, end)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return &booksv1.GetBooksProgressResponse{
		Progress: connect.ProgressResponse(labels, values, start, end),
	}, nil
}

func (h *tools) mcpSearchLibrary(
	ctx context.Context, args mcpQueryArgs,
) (proto.Message, error) {
	userID, err := authed(ctx)
	if err != nil {
		return nil, err
	}
	if args.Query == "" {
		return &booksv1.SearchLibraryResponse{Books: []*booksv1.UserBook{}}, nil
	}
	results, hasMore, err := h.books.SearchLibrary(ctx, userID, args.Query, 0, 0)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return &booksv1.SearchLibraryResponse{
		Books:   connect.ProtoUserBooks(results, h.coverBaseURL),
		HasMore: hasMore,
	}, nil
}

func (h *tools) mcpSearchExternal(
	ctx context.Context, args mcpQueryArgs,
) (proto.Message, error) {
	if _, err := authed(ctx); err != nil {
		return nil, err
	}
	if args.Query == "" {
		return &booksv1.SearchExternalResponse{
			Results: []*booksv1.ExternalBookResult{},
		}, nil
	}
	results := services.FilterExternalByQuery(
		args.Query, h.books.SearchExternal(ctx, args.Query),
	)
	return connect.ExternalBooksResponse(results), nil
}

func (h *tools) mcpGetSeries(
	ctx context.Context, args mcpSeriesArgs,
) (proto.Message, error) {
	userID, err := authed(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(args.Name)
	if name == "" {
		return nil, connect.MapError(services.ErrSeriesNameRequired)
	}
	view, err := h.books.GetSeries(ctx, userID, name)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.SeriesResponse(view, h.coverBaseURL), nil
}

func (h *tools) mcpGetExternalBook(
	ctx context.Context, args mcpExternalBookArgs,
) (proto.Message, error) {
	if _, err := authed(ctx); err != nil {
		return nil, err
	}
	book, err := h.books.GetExternal(ctx, args.Provider, args.ProviderID)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.ExternalBookResponse(*book), nil
}

func (h *tools) mcpGetReadingState(
	ctx context.Context, args mcpBookIDArgs,
) (proto.Message, error) {
	userID, bookID, err := authedBook(ctx, args.BookID)
	if err != nil {
		return nil, err
	}
	view, err := h.library.ReadingState(ctx, userID, bookID)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.ReadingStateResponse(view), nil
}

func (h *tools) mcpListResyncProposals(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	proposals, err := h.catalog.ListResyncProposals(ctx, caller(ctx))
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.ResyncProposalsResponse(proposals), nil
}

func (h *tools) mcpGetBookSources(
	ctx context.Context, args mcpBookSourcesArgs,
) (proto.Message, error) {
	user := caller(ctx)
	if err := services.RequireAdmin(user); err != nil {
		return nil, connect.MapError(err)
	}
	bookID, err := connect.ParseBookID(args.BookID)
	if err != nil {
		return nil, err
	}
	proposal, err := h.catalog.GetBookSources(
		ctx, user, bookID, args.OverrideTitle, args.OverrideAuthor,
	)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.BookSourcesResponse(proposal), nil
}

func (h *tools) mcpGetSourceStats(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	stats, err := h.catalog.GetSourceStats(ctx, caller(ctx))
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.SourceStatsResponse(stats), nil
}

func (h *tools) mcpListBooksInExactSources(
	ctx context.Context, args mcpSourcesArgs,
) (proto.Message, error) {
	books, err := h.catalog.ListBooksInExactSources(ctx, caller(ctx), args.Sources)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.BooksInExactSourcesResponse(books, h.coverBaseURL), nil
}

func (h *tools) mcpFindDuplicates(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	groups, err := h.catalog.FindDuplicates(ctx, caller(ctx))
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.DuplicatesResponse(groups, h.coverBaseURL), nil
}

func (h *tools) mcpGetBookFile(
	ctx context.Context, args mcpBookFileArgs,
) (proto.Message, error) {
	userID, bookID, err := authedBook(ctx, args.BookID)
	if err != nil {
		return nil, err
	}
	result, err := h.books.GetBookFile(ctx, userID, bookID, args.Format)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.BookFileResponse(result), nil
}

func (h *tools) mcpGetKEPUBStatus(
	ctx context.Context, args mcpBookIDArgs,
) (proto.Message, error) {
	userID, bookID, err := authedBook(ctx, args.BookID)
	if err != nil {
		return nil, err
	}
	view, err := h.library.KEPUBStatus(ctx, userID, bookID)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.KEPUBStatusResponse(view), nil
}

func (h *tools) mcpListKoboDevices(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := authed(ctx)
	if err != nil {
		return nil, err
	}
	devices, err := h.library.KoboDevices(ctx, userID)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.KoboDevicesResponse(devices), nil
}

func (h *tools) mcpListKoboStoreBooks(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := authed(ctx)
	if err != nil {
		return nil, err
	}
	books, err := h.books.ListKoboStoreBooks(ctx, userID)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.KoboStoreBooksResponse(books), nil
}

func (h *tools) mcpGetKoboDeviceLogs(
	ctx context.Context, args mcpDeviceIDArgs,
) (proto.Message, error) {
	userID, err := authed(ctx)
	if err != nil {
		return nil, err
	}
	deviceID, err := uuid.Parse(args.ID)
	if err != nil {
		return nil, connect.MapError(services.ErrInvalidDeviceID)
	}
	entries, err := h.library.KoboDeviceLogs(ctx, userID, deviceID)
	if err != nil {
		return nil, connect.MapError(err)
	}
	return connect.KoboLogsResponse(entries), nil
}

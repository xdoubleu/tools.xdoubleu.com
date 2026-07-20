package reading

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "reading"

type mcpProgressArgs struct {
	DateStart string `json:"date_start,omitempty" jsonschema:"window start (YYYY-MM-DD)"`
	DateEnd   string `json:"date_end,omitempty"   jsonschema:"window end (YYYY-MM-DD)"`
}

type mcpQueryArgs struct {
	Query string `json:"query" jsonschema:"search query"`
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

// RegisterMCPTools exposes the reading app's read-only RPCs on the combined apps
// MCP server. Every tool returns the calling user's own library data.
func (a *Reading) RegisterMCPTools(srv *mcp.Server) {
	h := &booksConnectHandler{app: a}
	registerReadingLibraryTools(srv, h)
	registerReadingCatalogTools(srv, h)
}

func registerReadingLibraryTools(srv *mcp.Server, h *booksConnectHandler) {
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_library",
		"The user's full library: reading/wishlist/finished shelves plus RSS.",
		h.mcpGetLibrary)
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_books_progress",
		"Reading-progress graph data over a date window.", h.mcpGetBooksProgress)
	mcptools.AddReadTool(srv, mcpAppName, "reading_search_library",
		"Search the user's library by text.", h.mcpSearchLibrary)
	mcptools.AddReadTool(srv, mcpAppName, "reading_search_external",
		"Search external metadata providers for a book.", h.mcpSearchExternal)
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_external_book",
		"Fetch one external provider's metadata for a book.",
		h.mcpGetExternalBook)
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_reading_state",
		"The latest reading position for a book.", h.mcpGetReadingState)
	mcptools.AddReadTool(srv, mcpAppName, "reading_list_feeds",
		"The user's RSS/Atom feed subscriptions.", h.mcpListFeeds)
}

func registerReadingCatalogTools(srv *mcp.Server, h *booksConnectHandler) {
	mcptools.AddReadTool(srv, mcpAppName, "reading_list_resync_proposals",
		"Catalog books that differ from an external source (resync wizard).",
		h.mcpListResyncProposals)
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_book_sources",
		"Live-fetch one book's candidates from every configured source.",
		h.mcpGetBookSources)
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_source_stats",
		"Per-source metadata coverage over the whole catalog.",
		h.mcpGetSourceStats)
	mcptools.AddReadTool(srv, mcpAppName, "reading_list_books_in_exact_sources",
		"Catalog books found by exactly the given set of sources.",
		h.mcpListBooksInExactSources)
	mcptools.AddReadTool(srv, mcpAppName, "reading_find_duplicates",
		"Groups of library entries judged to be the same book.",
		h.mcpFindDuplicates)
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_book_file",
		"A time-limited download URL for a book's file.", h.mcpGetBookFile)
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_kepub_status",
		"Whether a book has EPUB/PDF and its KEPUB conversion status.",
		h.mcpGetKEPUBStatus)
	mcptools.AddReadTool(srv, mcpAppName, "reading_list_kobo_devices",
		"The user's registered Kobo devices.", h.mcpListKoboDevices)
	mcptools.AddReadTool(srv, mcpAppName, "reading_get_kobo_device_logs",
		"Captured request/response logs for a Kobo device.",
		h.mcpGetKoboDeviceLogs)
}

func mcpOptStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *booksConnectHandler) mcpGetLibrary(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetLibrary(ctx, connect.NewRequest(
		&readingv1.GetLibraryRequest{},
	)))
}

func (h *booksConnectHandler) mcpGetBooksProgress(
	ctx context.Context, args mcpProgressArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetBooksProgress(ctx, connect.NewRequest(
		&readingv1.GetBooksProgressRequest{
			DateStart: args.DateStart,
			DateEnd:   args.DateEnd,
		},
	)))
}

func (h *booksConnectHandler) mcpSearchLibrary(
	ctx context.Context, args mcpQueryArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.SearchLibrary(ctx, connect.NewRequest(
		&readingv1.SearchLibraryRequest{Query: args.Query},
	)))
}

func (h *booksConnectHandler) mcpSearchExternal(
	ctx context.Context, args mcpQueryArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.SearchExternal(ctx, connect.NewRequest(
		&readingv1.SearchExternalRequest{Query: args.Query},
	)))
}

func (h *booksConnectHandler) mcpGetExternalBook(
	ctx context.Context, args mcpExternalBookArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetExternalBook(ctx, connect.NewRequest(
		&readingv1.GetExternalBookRequest{
			Provider:   args.Provider,
			ProviderId: args.ProviderID,
		},
	)))
}

func (h *booksConnectHandler) mcpGetReadingState(
	ctx context.Context, args mcpBookIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetReadingState(ctx, connect.NewRequest(
		&readingv1.GetReadingStateRequest{BookId: args.BookID},
	)))
}

func (h *booksConnectHandler) mcpListFeeds(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListFeeds(ctx, connect.NewRequest(
		&readingv1.ListFeedsRequest{},
	)))
}

func (h *booksConnectHandler) mcpListResyncProposals(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListResyncProposals(ctx, connect.NewRequest(
		&readingv1.ListResyncProposalsRequest{},
	)))
}

func (h *booksConnectHandler) mcpGetBookSources(
	ctx context.Context, args mcpBookSourcesArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetBookSources(ctx, connect.NewRequest(
		&readingv1.GetBookSourcesRequest{
			BookId:         args.BookID,
			OverrideTitle:  mcpOptStr(args.OverrideTitle),
			OverrideAuthor: mcpOptStr(args.OverrideAuthor),
		},
	)))
}

func (h *booksConnectHandler) mcpGetSourceStats(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetSourceStats(ctx, connect.NewRequest(
		&readingv1.GetSourceStatsRequest{},
	)))
}

func (h *booksConnectHandler) mcpListBooksInExactSources(
	ctx context.Context, args mcpSourcesArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListBooksInExactSources(ctx, connect.NewRequest(
		&readingv1.ListBooksInExactSourcesRequest{Sources: args.Sources},
	)))
}

func (h *booksConnectHandler) mcpFindDuplicates(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.FindDuplicates(ctx, connect.NewRequest(
		&readingv1.FindDuplicatesRequest{},
	)))
}

func (h *booksConnectHandler) mcpGetBookFile(
	ctx context.Context, args mcpBookFileArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetBookFile(ctx, connect.NewRequest(
		&readingv1.GetBookFileRequest{BookId: args.BookID, Format: args.Format},
	)))
}

func (h *booksConnectHandler) mcpGetKEPUBStatus(
	ctx context.Context, args mcpBookIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetKEPUBStatus(ctx, connect.NewRequest(
		&readingv1.GetKEPUBStatusRequest{BookId: args.BookID},
	)))
}

func (h *booksConnectHandler) mcpListKoboDevices(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListKoboDevices(ctx, connect.NewRequest(
		&readingv1.ListKoboDevicesRequest{},
	)))
}

func (h *booksConnectHandler) mcpGetKoboDeviceLogs(
	ctx context.Context, args mcpDeviceIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetKoboDeviceLogs(ctx, connect.NewRequest(
		&readingv1.GetKoboDeviceLogsRequest{Id: args.ID},
	)))
}

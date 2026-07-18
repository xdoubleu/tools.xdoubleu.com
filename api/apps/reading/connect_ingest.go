package reading

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/services"
	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// AddBookByURL ingests a pasted URL as a paper (arXiv) or article (anything
// else). Duplicates are not an error: the item is attached to the caller's
// library and already_in_library reports whether it was there before.
func (h *booksConnectHandler) AddBookByURL(
	ctx context.Context,
	req *connect.Request[readingv1.AddBookByURLRequest],
) (*connect.Response[readingv1.AddBookByURLResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}

	rawURL := strings.TrimSpace(req.Msg.Url)
	if rawURL == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("url is required"),
		)
	}
	override := req.Msg.Category
	if override != "" && override != models.CategoryPaper &&
		override != models.CategoryArticle {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New(`category must be "paper" or "article"`),
		)
	}

	result, err := h.app.Services.Ingest.AddByURL(ctx, user.ID, rawURL, override)
	if err != nil {
		return nil, ingestErrorToConnect(err)
	}

	return connect.NewResponse(&readingv1.AddBookByURLResponse{
		UserBook: protoUserBook(
			*result.UserBook, h.app.clients.PublicAPIBaseURL,
		),
		AlreadyInLibrary: result.AlreadyInLibrary,
	}), nil
}

// ingestErrorToConnect maps ingest failures onto Connect codes: user-fixable
// input problems become InvalidArgument/NotFound, upstream fetch problems
// become Unavailable (best-effort fetching, surfaced honestly).
func ingestErrorToConnect(err error) *connect.Error {
	switch {
	case errors.Is(err, services.ErrUnsupportedURL),
		errors.Is(err, services.ErrNoReadableContent),
		errors.Is(err, services.ErrNotAPDF),
		errors.Is(err, webfetch.ErrScheme):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, arxiv.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, webfetch.ErrTooLarge):
		return connect.NewError(connect.CodeResourceExhausted, err)
	case errors.Is(err, webfetch.ErrStatus),
		errors.Is(err, webfetch.ErrNetwork):
		return connect.NewError(connect.CodeUnavailable, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

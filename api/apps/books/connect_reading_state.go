package books

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) UpdateReadingProgress(
	ctx context.Context,
	req *connect.Request[booksv1.UpdateReadingProgressRequest],
) (*connect.Response[booksv1.UpdateReadingProgressResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	position, err := readingPositionFromProto(req.Msg.Position)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var readAt *time.Time
	if req.Msg.ReadAt != "" {
		t, parseErr := time.Parse(time.RFC3339Nano, req.Msg.ReadAt)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				errors.New("read_at must be RFC3339"),
			)
		}
		readAt = &t
	}
	var location *string
	if req.Msg.Location != "" {
		location = &req.Msg.Location
	}
	err = h.app.Services.Books.UpdateReadingProgress(
		ctx,
		models.BookReadingState{ //nolint:exhaustruct //UpdatedAt set by DB
			UserID:   user.ID,
			BookID:   bookID,
			Source:   req.Msg.Source,
			Percent:  int(req.Msg.Percent),
			Location: location,
			Position: position,
			ReadAt:   readAt,
		},
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&booksv1.UpdateReadingProgressResponse{}), nil
}

// GetReadingState returns an empty state when the book has none yet.
func (h *booksConnectHandler) GetReadingState(
	ctx context.Context,
	req *connect.Request[booksv1.GetReadingStateRequest],
) (*connect.Response[booksv1.GetReadingStateResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	state, err := h.app.Services.Books.GetReadingState(ctx, user.ID, bookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return connect.NewResponse(&booksv1.GetReadingStateResponse{}), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if state.Position != nil {
		pos := h.app.Services.Positions.ReaderPosition(
			ctx, user.ID, bookID, *state.Position,
		)
		state.Position = &pos
	}
	return connect.NewResponse(&booksv1.GetReadingStateResponse{
		State: readingStateToProto(state),
	}), nil
}

// TranslateReadingPosition adds the forms GetReadingState would.
func (h *booksConnectHandler) TranslateReadingPosition(
	ctx context.Context,
	req *connect.Request[booksv1.TranslateReadingPositionRequest],
) (*connect.Response[booksv1.TranslateReadingPositionResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	position, err := readingPositionFromProto(req.Msg.Position)
	if err == nil && position == nil {
		err = errors.New("position is required")
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	pos := h.app.Services.Positions.ReaderPosition(ctx, user.ID, bookID, *position)
	return connect.NewResponse(&booksv1.TranslateReadingPositionResponse{
		Position: readingPositionToProto(pos),
	}), nil
}

func readingPositionToProto(p models.ReadingPosition) *booksv1.ReadingPosition {
	return &booksv1.ReadingPosition{
		Href:   p.Href,
		Offset: int32FromInt(p.Offset),
		Page:   int32FromInt(p.Page),
	}
}

func readingStateToProto(state *models.BookReadingState) *booksv1.BookReadingStateData {
	out := &booksv1.BookReadingStateData{
		Source:    state.Source,
		Percent:   int32FromInt(state.Percent),
		Location:  stringPtr(state.Location),
		UpdatedAt: state.UpdatedAt.Format(time.RFC3339),
	}
	if state.Position != nil {
		out.Position = readingPositionToProto(*state.Position)
	}
	if state.ReadAt != nil {
		out.ReadAt = state.ReadAt.UTC().Format(time.RFC3339)
	}
	return out
}

// readingPositionFromProto accepts nil, {href, offset >= 0} or {page >= 1}.
func readingPositionFromProto(
	p *booksv1.ReadingPosition,
) (*models.ReadingPosition, error) {
	if p == nil {
		return nil, nil //nolint:nilnil // no position is valid
	}
	switch {
	case p.Page > 0 && p.Href == "" && p.Offset == 0:
		return &models.ReadingPosition{Href: "", Offset: 0, Page: int(p.Page)}, nil
	case p.Href != "" && p.Page == 0 && p.Offset >= 0:
		return &models.ReadingPosition{
			Href: p.Href, Offset: int(p.Offset), Page: 0,
		}, nil
	default:
		return nil, errors.New("position needs href with offset >= 0, or page >= 1")
	}
}

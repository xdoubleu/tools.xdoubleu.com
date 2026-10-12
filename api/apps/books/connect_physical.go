package books

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// GetPhysicalCopy returns the details of the user's physical copy.
func (h *booksConnectHandler) GetPhysicalCopy(
	ctx context.Context,
	req *booksv1.GetPhysicalCopyRequest,
) (*booksv1.GetPhysicalCopyResponse, error) {
	userID, bookID, err := physicalCopyCaller(ctx, req.BookId)
	if err != nil {
		return nil, err
	}
	view, err := h.app.Services.Physical.Copy(ctx, userID, bookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	out := &booksv1.GetPhysicalCopyResponse{
		Copy:          physicalCopyToProto(view.Copy),
		Syncable:      view.Syncable,
		HasPrintPages: view.HasPrintPages,
		Chapters:      make([]*booksv1.PhysicalChapter, 0, len(view.Chapters)),
	}
	for _, ch := range view.Chapters {
		out.Chapters = append(out.Chapters, &booksv1.PhysicalChapter{
			Title: ch.Title, Position: readingPositionToProto(ch.Position),
		})
	}
	return out, nil
}

// UpdatePhysicalCopy stores the details of the user's physical copy.
func (h *booksConnectHandler) UpdatePhysicalCopy(
	ctx context.Context,
	req *booksv1.UpdatePhysicalCopyRequest,
) (*booksv1.UpdatePhysicalCopyResponse, error) {
	userID, bookID, err := physicalCopyCaller(ctx, req.BookId)
	if err != nil {
		return nil, err
	}
	c := models.PhysicalCopy{
		PageCount:     int(req.GetCopy().GetPageCount()),
		TextStartPage: int(req.GetCopy().GetTextStartPage()),
		Anchors:       nil,
		Syncable:      false,
	}
	for _, a := range req.GetCopy().GetAnchors() {
		c.Anchors = append(c.Anchors, models.PhysicalAnchor{
			Page:   int(a.GetPage()),
			Href:   a.GetPosition().GetHref(),
			Offset: int(a.GetPosition().GetOffset()),
		})
	}
	err = h.app.Services.Physical.UpdateCopy(ctx, userID, bookID, c)
	switch {
	case errors.Is(err, services.ErrInvalidPhysicalCopy):
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	case errors.Is(err, database.ErrResourceNotFound):
		return nil, connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return &booksv1.UpdatePhysicalCopyResponse{}, nil
}

func physicalCopyCaller(
	ctx context.Context,
	rawBookID string,
) (string, uuid.UUID, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", uuid.Nil, connect.NewError(
			connect.CodeUnauthenticated, "unauthorized",
		)
	}
	bookID, err := uuid.Parse(rawBookID)
	if err != nil {
		return "", uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument, "invalid book ID",
		)
	}
	return user.ID, bookID, nil
}

func physicalCopyToProto(c models.PhysicalCopy) *booksv1.PhysicalCopy {
	out := &booksv1.PhysicalCopy{
		PageCount:     int32FromInt(c.PageCount),
		TextStartPage: int32FromInt(c.TextStartPage),
		Anchors:       make([]*booksv1.PhysicalAnchor, 0, len(c.Anchors)),
	}
	for _, a := range c.Anchors {
		out.Anchors = append(out.Anchors, &booksv1.PhysicalAnchor{
			Page: int32FromInt(a.Page),
			Position: readingPositionToProto(models.ReadingPosition{
				Href: a.Href, Offset: a.Offset, Page: 0,
			}),
		})
	}
	return out
}

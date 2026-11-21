package connect

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/movies/internal/protomap"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

// parseDay reads a YYYY-MM-DD watch date; "" is an unknown date (nil).
func parseDay(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil //nolint:nilnil // nil is the unknown date
	}
	day, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "date must be YYYY-MM-DD",
		)
	}
	return &day, nil
}

func optionalInt(n *int32) *int {
	if n == nil {
		return nil
	}
	v := int(*n)
	return &v
}

// target resolves the list owner and entry id of an entry RPC.
func (h *Handler) target(
	ctx context.Context,
	shared bool,
	rawID string,
) (string, uuid.UUID, error) {
	uid, err := h.owner(ctx, shared)
	if err != nil {
		return "", uuid.Nil, err
	}
	id, err := parseID(rawID)
	return uid, id, err
}

func (h *Handler) SetSeasonWatched(
	ctx context.Context,
	req *moviesv1.SetSeasonWatchedRequest,
) (*moviesv1.SetSeasonWatchedResponse, error) {
	uid, id, err := h.target(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	entry, err := h.movies.SetSeasonWatched(
		ctx, uid, id, int(req.SeasonNumber), req.Watched, req.UnknownDate,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetSeasonWatchedResponse{
		Entry:   protomap.Entry(entry),
		Seasons: protomap.Seasons(entry.Seasons),
	}, nil
}

func (h *Handler) AddWatchDate(
	ctx context.Context,
	req *moviesv1.AddWatchDateRequest,
) (*moviesv1.AddWatchDateResponse, error) {
	uid, id, err := h.target(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	day, err := parseDay(req.Date)
	if err != nil {
		return nil, err
	}
	entry, err := h.movies.AddWatchDate(
		ctx, uid, id, optionalInt(req.SeasonNumber), day,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.AddWatchDateResponse{
		Entry:   protomap.Entry(entry),
		Seasons: protomap.Seasons(entry.Seasons),
	}, nil
}

func (h *Handler) EditWatchDate(
	ctx context.Context,
	req *moviesv1.EditWatchDateRequest,
) (*moviesv1.EditWatchDateResponse, error) {
	uid, id, err := h.target(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	day, err := parseDay(req.Date)
	if err != nil {
		return nil, err
	}
	entry, err := h.movies.EditWatchDate(
		ctx, uid, id, optionalInt(req.SeasonNumber), int(req.Index), day,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.EditWatchDateResponse{
		Entry:   protomap.Entry(entry),
		Seasons: protomap.Seasons(entry.Seasons),
	}, nil
}

func (h *Handler) RemoveWatchDate(
	ctx context.Context,
	req *moviesv1.RemoveWatchDateRequest,
) (*moviesv1.RemoveWatchDateResponse, error) {
	uid, id, err := h.target(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	entry, err := h.movies.RemoveWatchDate(
		ctx, uid, id, optionalInt(req.SeasonNumber), int(req.Index),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.RemoveWatchDateResponse{
		Entry:   protomap.Entry(entry),
		Seasons: protomap.Seasons(entry.Seasons),
	}, nil
}

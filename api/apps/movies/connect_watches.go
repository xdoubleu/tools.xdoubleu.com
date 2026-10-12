package movies

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

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

// target resolves the caller and entry id of an entry RPC.
func target(ctx context.Context, rawID string) (string, uuid.UUID, error) {
	uid, err := userID(ctx)
	if err != nil {
		return "", uuid.Nil, err
	}
	id, err := parseID(rawID)
	return uid, id, err
}

func (h *moviesConnectHandler) SetSeasonWatched(
	ctx context.Context,
	req *moviesv1.SetSeasonWatchedRequest,
) (*moviesv1.SetSeasonWatchedResponse, error) {
	uid, id, err := target(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	entry, err := h.app.services.Movies.SetSeasonWatched(
		ctx, uid, id, int(req.SeasonNumber), req.Watched, req.UnknownDate,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetSeasonWatchedResponse{
		Entry:   protoEntry(entry),
		Seasons: protoSeasons(entry.Seasons),
	}, nil
}

func (h *moviesConnectHandler) AddWatchDate(
	ctx context.Context,
	req *moviesv1.AddWatchDateRequest,
) (*moviesv1.AddWatchDateResponse, error) {
	uid, id, err := target(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	day, err := parseDay(req.Date)
	if err != nil {
		return nil, err
	}
	entry, err := h.app.services.Movies.AddWatchDate(
		ctx, uid, id, optionalInt(req.SeasonNumber), day,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.AddWatchDateResponse{
		Entry:   protoEntry(entry),
		Seasons: protoSeasons(entry.Seasons),
	}, nil
}

func (h *moviesConnectHandler) EditWatchDate(
	ctx context.Context,
	req *moviesv1.EditWatchDateRequest,
) (*moviesv1.EditWatchDateResponse, error) {
	uid, id, err := target(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	day, err := parseDay(req.Date)
	if err != nil {
		return nil, err
	}
	entry, err := h.app.services.Movies.EditWatchDate(
		ctx, uid, id, optionalInt(req.SeasonNumber), int(req.Index), day,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.EditWatchDateResponse{
		Entry:   protoEntry(entry),
		Seasons: protoSeasons(entry.Seasons),
	}, nil
}

func (h *moviesConnectHandler) RemoveWatchDate(
	ctx context.Context,
	req *moviesv1.RemoveWatchDateRequest,
) (*moviesv1.RemoveWatchDateResponse, error) {
	uid, id, err := target(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	entry, err := h.app.services.Movies.RemoveWatchDate(
		ctx, uid, id, optionalInt(req.SeasonNumber), int(req.Index),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.RemoveWatchDateResponse{
		Entry:   protoEntry(entry),
		Seasons: protoSeasons(entry.Seasons),
	}, nil
}

package app

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"
)

// ScrubInternalErrors returns a Connect handler option that logs the original
// error of CodeInternal and CodeUnknown responses and replaces the message
// sent to the client with a generic one, so internal details never leak.
func ScrubInternalErrors(logger *slog.Logger) connect.Option {
	return connect.WithInterceptors(scrubInterceptor(logger))
}

func scrubInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}

			code := connect.CodeOf(err)
			if code != connect.CodeInternal && code != connect.CodeUnknown {
				return resp, err
			}

			logger.ErrorContext(
				ctx,
				"internal error",
				slog.String("procedure", req.Spec().Procedure),
				essentialogger.ErrAttr(err),
			)
			return resp, connect.NewError(code, errors.New("internal server error"))
		}
	}
}

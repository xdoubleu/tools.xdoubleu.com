package app

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect/v2"

	essentialogger "tools.xdoubleu.com/internal/logging"
)

// ScrubInternalErrors logs CodeInternal/CodeUnknown errors and replaces the
// client-facing message with a generic one so internals never leak.
func ScrubInternalErrors(logger *slog.Logger) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(
			ctx context.Context,
			spec connect.Spec,
			stream connect.ServerStream,
		) error {
			err := next(ctx, spec, stream)
			if err == nil {
				return nil
			}

			if errors.Is(err, context.Canceled) {
				return connect.NewError(connect.CodeCanceled, err.Error()).WithCause(err)
			}

			code := connect.CodeOf(err)
			if code != connect.CodeInternal && code != connect.CodeUnknown {
				return err
			}

			logger.ErrorContext(
				ctx,
				"internal error",
				slog.String("procedure", spec.Procedure),
				essentialogger.ErrAttr(err),
			)
			return connect.NewError(code, "internal server error")
		}
	}
}

package services

import (
	"context"
	"log/slog"

	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth auth.Service
	Room *RoomService
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	authService auth.Service,
) *Services {
	return &Services{
		Auth: authService,
		Room: NewRoomService(ctx, logger),
	}
}

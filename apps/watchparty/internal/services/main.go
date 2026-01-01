package services

import (
	"log/slog"

	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth auth.Service
	Room *RoomService
}

func New(
	logger *slog.Logger,
	authService auth.Service,
) *Services {
	return &Services{
		Auth: authService,
		Room: NewRoomService(logger),
	}
}

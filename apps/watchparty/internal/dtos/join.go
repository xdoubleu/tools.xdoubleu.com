package dtos

import "github.com/xdoubleu/essentia/v3/pkg/validate"

type JoinRoomDto struct {
	RoomCode string `schema:"roomCode"`
}

func (dto *JoinRoomDto) Validate() (bool, map[string]string) {
	v := validate.New()
	validate.Check(v, "roomCode", dto.RoomCode, validate.IsNotEmpty)
	return v.Valid(), v.Errors()
}

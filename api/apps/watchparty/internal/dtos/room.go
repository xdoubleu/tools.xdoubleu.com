package dtos

type Role string

const (
	Presenter Role = "presenter"
	Viewer    Role = "viewer"
)

type ActiveRoomDto struct {
	RoomCode string `json:"roomCode"`
	Role     Role   `json:"role"`
}

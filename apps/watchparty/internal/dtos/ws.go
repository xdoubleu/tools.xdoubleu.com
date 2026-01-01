package dtos

import (
	"encoding/json"

	"github.com/XDoubleU/essentia/pkg/validate"
)

type SubscribeMessageDto struct {
	RoomCode string `json:"roomCode"`
	Role     Role   `json:"role"`
}

func (dto SubscribeMessageDto) Topic() string {
	return ""
}

func (dto SubscribeMessageDto) Validate() (bool, map[string]string) {
	v := validate.New()

	validate.Check(v, "roomCode", dto.RoomCode, validate.IsNotEmpty)
	validate.Check(v, "role", dto.Role, validate.IsInSlice([]Role{Presenter, Viewer}))

	return v.Valid(), v.Errors()
}

type Type string

const (
	Offer     Type = "offer"
	Answer    Type = "answer"
	Candidate Type = "candidate"
)

type TrackMessage struct {
	Type      Type            `json:"type"`      // offer, answer, candidate
	Payload   json.RawMessage `json:"payload"`   // SDP or ICE
	TrackType string          `json:"trackType"` // screen, camera, microphone
}

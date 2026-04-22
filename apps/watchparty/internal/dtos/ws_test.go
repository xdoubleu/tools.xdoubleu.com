package dtos_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
)

func TestSubscribeMessageDtoTopic(t *testing.T) {
	dto := dtos.SubscribeMessageDto{RoomCode: "ABC123", Role: dtos.Viewer}
	assert.Equal(t, "", dto.Topic())
}

func TestSubscribeMessageDtoValidateValid(t *testing.T) {
	dto := dtos.SubscribeMessageDto{RoomCode: "ABC123", Role: dtos.Presenter}
	ok, errs := dto.Validate()
	assert.True(t, ok)
	assert.Empty(t, errs)
}

func TestSubscribeMessageDtoValidateInvalidRole(t *testing.T) {
	dto := dtos.SubscribeMessageDto{RoomCode: "ABC123", Role: "superuser"}
	ok, errs := dto.Validate()
	assert.False(t, ok)
	assert.Contains(t, errs, "role")
}

func TestSubscribeMessageDtoValidateEmptyRoomCode(t *testing.T) {
	dto := dtos.SubscribeMessageDto{RoomCode: "", Role: dtos.Viewer}
	ok, errs := dto.Validate()
	assert.False(t, ok)
	assert.Contains(t, errs, "roomCode")
}

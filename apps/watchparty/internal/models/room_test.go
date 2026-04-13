package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	"tools.xdoubleu.com/apps/watchparty/internal/models"
)

func makeOffer(trackType string) dtos.TrackMessage {
	payload, _ := json.Marshal(map[string]string{"type": "offer", "sdp": "v=0"})
	return dtos.TrackMessage{
		Type:      dtos.Offer,
		Payload:   payload,
		TrackType: trackType,
	}
}

func TestNewRoom(t *testing.T) {
	before := time.Now()
	room := models.NewRoom("user-1")
	after := time.Now()

	assert.Equal(t, "user-1", room.Presenter.ID)
	assert.Empty(t, room.Presenter.WS)
	assert.Empty(t, room.Viewer.ID)
	assert.True(t, !room.LastActive.Before(before) && !room.LastActive.After(after))
}

func TestSetViewer(t *testing.T) {
	room := models.NewRoom("presenter-1")

	before := time.Now()
	room.SetViewer("viewer-1")
	after := time.Now()

	assert.Equal(t, "viewer-1", room.Viewer.ID)
	assert.True(t, !room.LastActive.Before(before) && !room.LastActive.After(after))
}

func TestRemoveViewer(t *testing.T) {
	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")

	room.RemoveViewer()

	assert.Empty(t, room.Viewer.ID)
	assert.Nil(t, room.Viewer.WS)
}

func TestSendToViewerBuffersWhenNoWS(t *testing.T) {
	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")
	// Viewer has no WS yet — message must be buffered without error.

	err := room.SendToViewer(t.Context(), makeOffer("cam"))
	assert.NoError(t, err)

	err = room.SendToViewer(t.Context(), makeOffer("screen"))
	assert.NoError(t, err)
}

func TestSendToPresenterBuffersWhenNoWS(t *testing.T) {
	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")

	err := room.SendToPresenter(t.Context(), makeOffer("cam"))
	assert.NoError(t, err)

	err = room.SendToPresenter(t.Context(), makeOffer("screen"))
	assert.NoError(t, err)
}

func TestLastActiveUpdatesOnSetViewer(t *testing.T) {
	room := models.NewRoom("presenter-1")
	before := room.LastActive

	time.Sleep(time.Millisecond)
	room.SetViewer("viewer-1")

	assert.True(t, room.LastActive.After(before))
}

func TestLastActiveUpdatesOnRemoveViewer(t *testing.T) {
	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")
	before := room.LastActive

	time.Sleep(time.Millisecond)
	room.RemoveViewer()

	assert.True(t, room.LastActive.After(before))
}

func TestLastActiveUpdatesOnSendToViewer(t *testing.T) {
	room := models.NewRoom("presenter-1")
	before := room.LastActive

	time.Sleep(time.Millisecond)
	_ = room.SendToViewer(t.Context(), makeOffer("cam"))

	assert.True(t, room.LastActive.After(before))
}

func TestLastActiveUpdatesOnSendToPresenter(t *testing.T) {
	room := models.NewRoom("presenter-1")
	before := room.LastActive

	time.Sleep(time.Millisecond)
	_ = room.SendToPresenter(t.Context(), makeOffer("cam"))

	assert.True(t, room.LastActive.After(before))
}

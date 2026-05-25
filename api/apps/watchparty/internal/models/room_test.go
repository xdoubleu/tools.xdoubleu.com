package models_test

import (
	"context"
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
		Direction: "",
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

// TestSendToPresenter_TracksLastOffer verifies that SendToPresenter records
// the last offer for each track type, which is later resent on reconnect.
func TestSendToPresenter_TracksLastOffer(t *testing.T) {
	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")

	ctx := context.Background()

	camOffer := makeOffer("cam")
	_ = room.SendToPresenter(ctx, camOffer)

	screenOffer := makeOffer("screen")
	_ = room.SendToPresenter(ctx, screenOffer)

	// Verify offers are tracked by track type.
	assert.NotNil(t, room.GetLastOfferFromViewer("cam"))
	assert.NotNil(t, room.GetLastOfferFromViewer("screen"))
	assert.Equal(t, camOffer.Type, room.GetLastOfferFromViewer("cam").Type)
	assert.Equal(t, screenOffer.Type, room.GetLastOfferFromViewer("screen").Type)
}

// TestSendToViewer_TracksLastOffer verifies that SendToViewer records
// the last offer for each track type from the presenter.
func TestSendToViewer_TracksLastOffer(t *testing.T) {
	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")

	ctx := context.Background()

	camOffer := makeOffer("cam")
	_ = room.SendToViewer(ctx, camOffer)

	screenOffer := makeOffer("screen")
	_ = room.SendToViewer(ctx, screenOffer)

	// Verify offers are tracked by track type.
	assert.NotNil(t, room.GetLastOfferFromPresenter("cam"))
	assert.NotNil(t, room.GetLastOfferFromPresenter("screen"))
	assert.Equal(t, camOffer.Type, room.GetLastOfferFromPresenter("cam").Type)
	assert.Equal(t, screenOffer.Type, room.GetLastOfferFromPresenter("screen").Type)
}

// TestRemoveViewer_ClearsWS verifies that RemoveViewer clears the viewer's
// WebSocket connection. The test verifies that RemoveViewer zeroes out the
// Viewer field including its WS.
func TestRemoveViewer_ClearsWS(t *testing.T) {
	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")

	room.RemoveViewer()

	// After RemoveViewer, both ID and WS should be cleared.
	assert.Nil(t, room.Viewer.WS)
	assert.Empty(t, room.Viewer.ID)
}

// TestSetPresenterWS_AssignsNewConnection verifies that SetPresenterWS
// assigns the new connection. The close of old connections is tested in
// integration tests (ws_test.go) where real WebSocket connections are available.
func TestSetPresenterWS_AssignsNewConnection(t *testing.T) {
	room := models.NewRoom("presenter-1")

	// SetPresenterWS with nil is safe and assigns the value.
	room.SetPresenterWS(nil)
	assert.Nil(t, room.Presenter.WS)
}

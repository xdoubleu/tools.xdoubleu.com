package models

import (
	"context"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
)

type RoomUser struct {
	ID string
	WS *websocket.Conn
}

type Room struct {
	LastActive time.Time
	Presenter  RoomUser
	Viewer     RoomUser

	// buffered messages sent while the target has no WS connection
	pendingForPresenter []dtos.TrackMessage
	pendingForViewer    []dtos.TrackMessage

	// last offers from each side — resent to a reconnecting peer so it can
	// trigger a fresh full negotiation cycle on its recreated peer connections
	lastOfferFromPresenter map[string]*dtos.TrackMessage // key: "cam" / "screen"
	lastOfferFromViewer    map[string]*dtos.TrackMessage
}

func NewRoom(presenterID string) Room {
	//nolint:exhaustruct //other fields are optional
	return Room{
		LastActive: time.Now(),
		//nolint:exhaustruct //other fields are optional
		Presenter:              RoomUser{ID: presenterID},
		lastOfferFromPresenter: make(map[string]*dtos.TrackMessage),
		lastOfferFromViewer:    make(map[string]*dtos.TrackMessage),
	}
}

func (r *Room) updateLastActive() { r.LastActive = time.Now() }

// SetPresenterWS is called when the presenter's WebSocket connects or reconnects.
func (r *Room) SetPresenterWS(ws *websocket.Conn) {
	r.Presenter.WS = ws
	r.updateLastActive()

	// flush buffered messages
	for _, msg := range r.pendingForPresenter {
		_ = wsjson.Write(context.Background(), ws, msg)
	}
	r.pendingForPresenter = nil

	// resend the viewer's last offers so the presenter can renegotiate its
	// peer connections with fresh ICE credentials
	for _, offer := range r.lastOfferFromViewer {
		if offer != nil {
			_ = wsjson.Write(context.Background(), ws, offer)
		}
	}
}

// SetViewer records the viewer's user ID.
func (r *Room) SetViewer(viewerID string) {
	r.Viewer.ID = viewerID
	r.updateLastActive()
}

// SetViewerWS is called when the viewer's WebSocket connects or reconnects.
func (r *Room) SetViewerWS(ws *websocket.Conn) {
	r.Viewer.WS = ws
	r.updateLastActive()

	// flush buffered messages
	for _, msg := range r.pendingForViewer {
		_ = wsjson.Write(context.Background(), ws, msg)
	}
	r.pendingForViewer = nil

	// resend the presenter's last offers so the viewer can renegotiate its
	// peer connections with fresh ICE credentials
	for _, offer := range r.lastOfferFromPresenter {
		if offer != nil {
			_ = wsjson.Write(context.Background(), ws, offer)
		}
	}
}

func (r *Room) RemoveViewer() {
	//nolint:exhaustruct //other fields are optional
	r.Viewer = RoomUser{}
	r.updateLastActive()
}

func (r *Room) SendToViewer(ctx context.Context, trackMsg dtos.TrackMessage) error {
	r.updateLastActive()

	if trackMsg.Type == dtos.Offer {
		r.lastOfferFromPresenter[trackMsg.TrackType] = &trackMsg
	}

	if r.Viewer.WS == nil {
		r.pendingForViewer = append(r.pendingForViewer, trackMsg)
		return nil
	}
	return wsjson.Write(ctx, r.Viewer.WS, trackMsg)
}

func (r *Room) SendToPresenter(ctx context.Context, trackMsg dtos.TrackMessage) error {
	r.updateLastActive()

	if trackMsg.Type == dtos.Offer {
		r.lastOfferFromViewer[trackMsg.TrackType] = &trackMsg
	}

	if r.Presenter.WS == nil {
		r.pendingForPresenter = append(r.pendingForPresenter, trackMsg)
		return nil
	}
	return wsjson.Write(ctx, r.Presenter.WS, trackMsg)
}

package models

import (
	"context"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
)

type RoomUser struct {
	ID           string
	WS           *websocket.Conn
	ScreenActive bool
}

type Room struct {
	LastActive time.Time
	Presenter  RoomUser
	Viewer     RoomUser

	// buffered messages
	pendingForPresenter []dtos.TrackMessage
	pendingForViewer    []dtos.TrackMessage

	// store last offers to resend to reconnecting clients
	lastOfferFromPresenter map[string]*dtos.TrackMessage // key: "cam"/"screen"
	lastOfferFromViewer    map[string]*dtos.TrackMessage

	// store last answers
	lastAnswerFromPresenter map[string]*dtos.TrackMessage
	lastAnswerFromViewer    map[string]*dtos.TrackMessage
}

func NewRoom(presenterID string) Room {
	//nolint:exhaustruct //other fields are optional
	return Room{
		LastActive: time.Now(),
		//nolint:exhaustruct //other fields are optional
		Presenter:               RoomUser{ID: presenterID},
		lastOfferFromPresenter:  make(map[string]*dtos.TrackMessage),
		lastOfferFromViewer:     make(map[string]*dtos.TrackMessage),
		lastAnswerFromPresenter: make(map[string]*dtos.TrackMessage),
		lastAnswerFromViewer:    make(map[string]*dtos.TrackMessage),
	}
}

func (r *Room) updateLastActive() { r.LastActive = time.Now() }

// Presenter connects/reconnects.
func (r *Room) SetPresenterWS(ws *websocket.Conn) {
	r.Presenter.WS = ws
	r.updateLastActive()

	// flush pending
	for _, msg := range r.pendingForPresenter {
		_ = wsjson.Write(context.Background(), ws, msg)
	}
	r.pendingForPresenter = nil

	// resend last viewer offers & answers
	for _, offer := range r.lastOfferFromViewer {
		if offer != nil {
			_ = wsjson.Write(context.Background(), ws, offer)
		}
	}
	for _, ans := range r.lastAnswerFromViewer {
		if ans != nil {
			_ = wsjson.Write(context.Background(), ws, ans)
		}
	}
}

// Viewer sets ID.
func (r *Room) SetViewer(viewerID string) {
	r.Viewer.ID = viewerID
	r.updateLastActive()
}

// Viewer connects/reconnects.
func (r *Room) SetViewerWS(ws *websocket.Conn) {
	r.Viewer.WS = ws
	r.updateLastActive()

	// flush pending
	for _, msg := range r.pendingForViewer {
		_ = wsjson.Write(context.Background(), ws, msg)
	}
	r.pendingForViewer = nil

	// resend last presenter offers & answers
	for _, offer := range r.lastOfferFromPresenter {
		if offer != nil {
			_ = wsjson.Write(context.Background(), ws, offer)
		}
	}
	for _, ans := range r.lastAnswerFromPresenter {
		if ans != nil {
			_ = wsjson.Write(context.Background(), ws, ans)
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

	key := trackMsg.TrackType // "cam" or "screen"
	switch trackMsg.Type {
	case dtos.Offer:
		r.lastOfferFromPresenter[key] = &trackMsg
	case dtos.Answer:
		r.lastAnswerFromPresenter[key] = &trackMsg
	case dtos.Candidate:
		// do nothing
	}

	if r.Viewer.WS == nil {
		r.pendingForViewer = append(r.pendingForViewer, trackMsg)
		return nil
	}
	return wsjson.Write(ctx, r.Viewer.WS, trackMsg)
}

func (r *Room) SendToPresenter(ctx context.Context, trackMsg dtos.TrackMessage) error {
	r.updateLastActive()

	key := trackMsg.TrackType
	switch trackMsg.Type {
	case dtos.Offer:
		r.lastOfferFromViewer[key] = &trackMsg
	case dtos.Answer:
		r.lastAnswerFromViewer[key] = &trackMsg
	case dtos.Candidate:
		// do nothing
	}

	if r.Presenter.WS == nil {
		r.pendingForPresenter = append(r.pendingForPresenter, trackMsg)
		return nil
	}
	return wsjson.Write(ctx, r.Presenter.WS, trackMsg)
}

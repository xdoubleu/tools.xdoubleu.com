package watchparty

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	wstools "github.com/XDoubleU/essentia/pkg/communication/ws"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
)

func (app *WatchParty) wsRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		fmt.Sprintf("GET %s/signaling", prefix),
		app.services.Auth.Access(app.WsSignalingHandler()),
	)
}

func (app *WatchParty) WsSignalingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(
			w,
			r,
			//nolint:exhaustruct //other fields are optional
			&websocket.AcceptOptions{InsecureSkipVerify: true},
		)
		if err != nil {
			log.Printf("websocket accept error: %v", err)
			return
		}
		defer conn.Close(
			websocket.StatusNormalClosure,
			"closing connection",
		) // normal closure

		var msg dtos.SubscribeMessageDto
		err = wsjson.Read(r.Context(), conn, &msg)
		if err != nil {
			wstools.ServerErrorResponse(r.Context(), conn, err)
			return
		}

		if valid, errors := msg.Validate(); !valid {
			wstools.FailedValidationResponse(r.Context(), conn, errors)
			return
		}

		switch msg.Role {
		case dtos.Presenter:
			app.handlePresenter(r.Context(), conn, msg)
		case dtos.Viewer:
			app.handleViewer(r.Context(), conn, msg)
		}
	}
}

func (app *WatchParty) handlePresenter(
	ctx context.Context,
	conn *websocket.Conn,
	msg dtos.SubscribeMessageDto,
) {
	if !app.services.Room.JoinPresenter(msg.RoomCode, conn) {
		wstools.ServerErrorResponse(
			ctx,
			conn,
			errors.New("couldn't set presenter websocket"),
		)
		return
	}

	for {
		var trackMsg dtos.TrackMessage
		err := wsjson.Read(ctx, conn, &trackMsg)
		if err != nil {
			log.Printf("read error: %v", err)
			return
		}

		log.Printf(
			"⬅️ Received message (presenter) (%s) (%s)",
			trackMsg.Type,
			trackMsg.TrackType,
		)
		app.services.Room.SendToViewer(msg.RoomCode, trackMsg)
	}
}

func (app *WatchParty) handleViewer(
	ctx context.Context,
	conn *websocket.Conn,
	msg dtos.SubscribeMessageDto,
) {
	app.services.Room.JoinViewerWS(msg.RoomCode, conn)
	defer app.services.Room.LeaveViewer(msg.RoomCode)

	for {
		var trackMsg dtos.TrackMessage
		err := wsjson.Read(ctx, conn, &trackMsg)
		if err != nil {
			log.Printf("read error: %v", err)
			return
		}

		log.Printf(
			"⬅️ Received message (viewer) (%s) (%s)",
			trackMsg.Type,
			trackMsg.TrackType,
		)
		app.services.Room.SendToPresenter(msg.RoomCode, trackMsg)
	}
}

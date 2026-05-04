package watchparty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	wstools "github.com/xdoubleu/essentia/v4/pkg/communication/wstools"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
)

const (
	pingInterval = 30 * time.Second
	pingTimeout  = 10 * time.Second
)

func pingLoop(
	ctx context.Context,
	conn *websocket.Conn,
	interval, timeout time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, timeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func isExpectedCloseErr(err error) bool {
	status := websocket.CloseStatus(err)
	if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
		return true
	}
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}

func (app *WatchParty) wsRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		fmt.Sprintf("GET %s/signaling", prefix),
		app.Services.Auth.Access(app.WsSignalingHandler()),
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
			app.Logger.ErrorContext(
				r.Context(),
				"websocket accept error",
				slog.Any("err", err),
			)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "closing connection")

		var msg dtos.SubscribeMessageDto
		err = wsjson.Read(r.Context(), conn, &msg)
		if err != nil {
			wstools.ServerErrorResponse(r.Context(), conn, err)
			return
		}

		if valid, errs := msg.Validate(); !valid {
			wstools.FailedValidationResponse(r.Context(), conn, errs)
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
	if !app.Services.Room.JoinPresenter(ctx, msg.RoomCode, conn) {
		wstools.ServerErrorResponse(
			ctx,
			conn,
			errors.New("couldn't set presenter websocket"),
		)
		return
	}

	go pingLoop(ctx, conn, pingInterval, pingTimeout)

	for {
		var trackMsg dtos.TrackMessage
		if err := wsjson.Read(ctx, conn, &trackMsg); err != nil {
			if isExpectedCloseErr(err) {
				app.Logger.DebugContext(
					ctx,
					"presenter disconnected",
					slog.Any("err", err),
				)
			} else {
				app.Logger.ErrorContext(ctx, "presenter read error", slog.Any("err", err))
			}
			return
		}

		app.Logger.DebugContext(ctx, "received message",
			slog.String("role", "presenter"),
			slog.String("type", string(trackMsg.Type)),
			slog.String("trackType", trackMsg.TrackType),
		)
		app.Services.Room.SendToViewer(ctx, msg.RoomCode, trackMsg)
	}
}

func (app *WatchParty) handleViewer(
	ctx context.Context,
	conn *websocket.Conn,
	msg dtos.SubscribeMessageDto,
) {
	app.Services.Room.JoinViewerWS(ctx, msg.RoomCode, conn)
	defer app.Services.Room.LeaveViewer(ctx, msg.RoomCode)

	go pingLoop(ctx, conn, pingInterval, pingTimeout)

	for {
		var trackMsg dtos.TrackMessage
		if err := wsjson.Read(ctx, conn, &trackMsg); err != nil {
			if isExpectedCloseErr(err) {
				app.Logger.DebugContext(
					ctx,
					"viewer disconnected",
					slog.Any("err", err),
				)
			} else {
				app.Logger.ErrorContext(ctx, "viewer read error", slog.Any("err", err))
			}
			return
		}

		app.Logger.DebugContext(ctx, "received message",
			slog.String("role", "viewer"),
			slog.String("type", string(trackMsg.Type)),
			slog.String("trackType", trackMsg.TrackType),
		)
		app.Services.Room.SendToPresenter(ctx, msg.RoomCode, trackMsg)
	}
}

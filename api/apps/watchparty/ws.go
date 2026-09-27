package watchparty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	wstools "tools.xdoubleu.com/internal/communication/wstools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/models"
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
	if status == websocket.StatusNormalClosure ||
		status == websocket.StatusGoingAway ||
		status == websocket.StatusNoStatusRcvd {
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
		app.Services.Auth.AppAccess(app.GetName(), app.WsSignalingHandler()),
	)
}

func (app *WatchParty) WsSignalingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Same-host origins are always allowed; web runs on another port in dev.
		conn, err := websocket.Accept(
			w,
			r,
			//nolint:exhaustruct //other fields are optional
			&websocket.AcceptOptions{OriginPatterns: app.webOriginPatterns()},
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
			app.handlePresenter(r.Context(), conn, msg, user.ID)
		case dtos.Viewer:
			app.handleViewer(r.Context(), conn, msg, user.ID)
		}
	}
}

func (app *WatchParty) handlePresenter(
	ctx context.Context,
	conn *websocket.Conn,
	msg dtos.SubscribeMessageDto,
	userID string,
) {
	if !app.Services.Room.JoinPresenter(ctx, msg.RoomCode, userID, conn) {
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
	userID string,
) {
	if !app.Services.Room.JoinViewerWS(ctx, msg.RoomCode, userID, conn) {
		wstools.ServerErrorResponse(
			ctx,
			conn,
			errors.New("couldn't set viewer websocket"),
		)
		return
	}
	defer app.Services.Room.DisconnectViewer(msg.RoomCode, conn)

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

// webOriginPatterns allows the web app's host to open the signaling socket.
func (app *WatchParty) webOriginPatterns() []string {
	u, err := url.Parse(app.Config.WebURL)
	if err != nil || u.Host == "" {
		return nil
	}
	return []string{u.Host}
}

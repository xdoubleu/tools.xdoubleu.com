package books

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/books/internal/services"
)

// maxBodyCapture caps how many bytes of each request/response body are kept for
// debug logging. Bodies still flow through untouched to the handler and the
// upstream proxy; only the captured copy is truncated.
const maxBodyCapture = 64 * 1024

type koboLogCtxKey struct{}

// koboLogHolder accumulates the captured request/response for a single Kobo
// request. koboAuth fills in deviceID/enabled once the token is resolved; the
// tee reader and response recorder only capture while enabled is true.
type koboLogHolder struct {
	enabled  bool
	deviceID string
	reqBody  bytes.Buffer
	respBody bytes.Buffer
	status   int
}

// koboLogHolderFrom returns the holder installed by koboLogged, or nil.
func koboLogHolderFrom(ctx context.Context) *koboLogHolder {
	h, _ := ctx.Value(koboLogCtxKey{}).(*koboLogHolder)
	return h
}

// capWrite copies up to the remaining capacity of buf (bounded by
// maxBodyCapture) so a single body can never buffer more than the cap.
func capWrite(buf *bytes.Buffer, p []byte) {
	remaining := maxBodyCapture - buf.Len()
	if remaining <= 0 {
		return
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	buf.Write(p)
}

// koboBodyTee wraps the request body and, while the holder is enabled, copies
// read bytes into the holder's request buffer (capped). The full stream still
// reaches the handler/proxy unchanged.
type koboBodyTee struct {
	rc     io.ReadCloser
	holder *koboLogHolder
}

func (t *koboBodyTee) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n > 0 && t.holder.enabled {
		capWrite(&t.holder.reqBody, p[:n])
	}
	return n, err
}

func (t *koboBodyTee) Close() error { return t.rc.Close() }

// koboResponseRecorder wraps the ResponseWriter to capture the status code
// (always) and, while enabled, the response body (capped). All writes pass
// through to the real writer, so redirects and proxy io.Copy are unaffected.
type koboResponseRecorder struct {
	http.ResponseWriter
	holder *koboLogHolder
}

func (rec *koboResponseRecorder) WriteHeader(status int) {
	rec.holder.status = status
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *koboResponseRecorder) Write(p []byte) (int, error) {
	if rec.holder.enabled {
		capWrite(&rec.holder.respBody, p)
	}
	return rec.ResponseWriter.Write(p)
}

// redactKoboToken replaces the raw bearer token segment in a captured request
// path with a placeholder. The token is the device's live sync credential —
// it must never be persisted (even in the in-memory debug log the device
// owner can view) per the "plaintext never stored" rule documented in
// kobo_routes.go.
func redactKoboToken(path, token string) string {
	if token == "" {
		return path
	}
	return strings.Replace(path, "/"+token+"/", "/redacted/", 1)
}

// koboLogged wraps a Kobo device-facing handler so that, when debug logging is
// enabled for the authenticated device, the request endpoint + body and the
// response status + body are captured into the in-memory KoboLogStore.
func (app *Books) koboLogged(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//nolint:exhaustruct // zero values are the intended initial state
		holder := &koboLogHolder{status: http.StatusOK}
		ctx := context.WithValue(r.Context(), koboLogCtxKey{}, holder)
		r = r.WithContext(ctx)
		if r.Body != nil {
			r.Body = &koboBodyTee{rc: r.Body, holder: holder}
		}
		rec := &koboResponseRecorder{ResponseWriter: w, holder: holder}

		next(rec, r)

		if holder.enabled && holder.deviceID != "" {
			app.Services.KoboLog.Append(holder.deviceID, services.KoboLogEntry{
				Time:         time.Now(),
				Method:       r.Method,
				Path:         redactKoboToken(r.URL.Path, r.PathValue("token")),
				Query:        r.URL.RawQuery,
				RequestBody:  holder.reqBody.String(),
				Status:       holder.status,
				ResponseBody: holder.respBody.String(),
			})
		}
	}
}

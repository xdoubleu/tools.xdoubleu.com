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

// maxBodyCapture caps the captured copy of each body; the stream is untouched.
const maxBodyCapture = 64 * 1024

type koboLogCtxKey struct{}

// koboLogHolder collects one request's capture; koboAuth sets
// deviceID/enabled, and capture happens only while enabled.
type koboLogHolder struct {
	enabled  bool
	deviceID string
	reqBody  bytes.Buffer
	respBody bytes.Buffer
	status   int
}

func koboLogHolderFrom(ctx context.Context) *koboLogHolder {
	h, _ := ctx.Value(koboLogCtxKey{}).(*koboLogHolder)
	return h
}

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

// koboBodyTee copies read bytes into the holder while enabled.
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

// koboResponseRecorder always records the status and, while enabled, the
// body; writes pass through unchanged.
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

// redactKoboToken masks the token in a captured path: it is the device's live
// credential and must never be stored, even in the debug log.
func redactKoboToken(path, token string) string {
	if token == "" {
		return path
	}
	return strings.Replace(path, "/"+token+"/", "/redacted/", 1)
}

// koboLogged captures a device's requests and responses into KoboLogStore
// when debug logging is on for it.
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

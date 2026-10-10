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
	notes    string
	// wroteHeader marks respHeaders as snapshotted at the moment the
	// response headers were sent.
	wroteHeader     bool
	respHeaders     map[string]string
	upstreamHeaders map[string]string
}

// Sync paging headers safe to record on a debug-log entry. Credentials
// (Authorization, cookies) must never be added here.
//
//nolint:gochecknoglobals // static allowlists, never mutated
var (
	koboLogRequestHeaders  = []string{koboHeaderSyncToken}
	koboLogResponseHeaders = []string{
		koboHeaderSync, koboHeaderSyncMode, koboHeaderSyncToken, "content-encoding",
	}
)

// koboAllowedHeaders returns the allowlisted headers present in h, keyed by
// their lowercase name, or nil if none are.
func koboAllowedHeaders(h http.Header, allow []string) map[string]string {
	var out map[string]string
	for _, name := range allow {
		v := h.Get(name)
		if v == "" {
			continue
		}
		if out == nil {
			out = make(map[string]string, len(allow))
		}
		out[name] = v
	}
	return out
}

func koboLogHolderFrom(ctx context.Context) *koboLogHolder {
	h, _ := ctx.Value(koboLogCtxKey{}).(*koboLogHolder)
	return h
}

// koboSetUpstreamNote records the upstream store outcome for a request on its
// debug-log entry and as an x-kobo-upstream response header, making a
// silently-dropped store merge failure visible. An empty note is a no-op.
func koboSetUpstreamNote(r *http.Request, w http.ResponseWriter, note string) {
	if note == "" {
		return
	}
	if holder := koboLogHolderFrom(r.Context()); holder != nil {
		holder.notes = note
	}
	w.Header().Set("x-kobo-upstream", note)
}

// koboSetUpstreamHeaders records the upstream store response's allowlisted
// headers on the request's debug-log entry.
func koboSetUpstreamHeaders(r *http.Request, h http.Header) {
	if holder := koboLogHolderFrom(r.Context()); holder != nil {
		holder.upstreamHeaders = koboAllowedHeaders(h, koboLogResponseHeaders)
	}
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
// body and the headers as sent; writes pass through unchanged.
type koboResponseRecorder struct {
	http.ResponseWriter
	holder *koboLogHolder
}

func (rec *koboResponseRecorder) WriteHeader(status int) {
	rec.holder.status = status
	rec.snapshotHeaders()
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *koboResponseRecorder) Write(p []byte) (int, error) {
	rec.snapshotHeaders()
	if rec.holder.enabled {
		capWrite(&rec.holder.respBody, p)
	}
	return rec.ResponseWriter.Write(p)
}

// snapshotHeaders captures the response headers once, when they are sent;
// later Header() changes never reach the device.
func (rec *koboResponseRecorder) snapshotHeaders() {
	if rec.holder.wroteHeader {
		return
	}
	rec.holder.wroteHeader = true
	rec.holder.respHeaders = koboAllowedHeaders(rec.Header(), koboLogResponseHeaders)
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
		rec.snapshotHeaders()

		if holder.enabled && holder.deviceID != "" {
			app.Services.KoboLog.Append(holder.deviceID, services.KoboLogEntry{
				Time:         time.Now(),
				Method:       r.Method,
				Path:         redactKoboToken(r.URL.Path, r.PathValue("token")),
				Query:        r.URL.RawQuery,
				RequestBody:  holder.reqBody.String(),
				Status:       holder.status,
				ResponseBody: holder.respBody.String(),
				Notes:        holder.notes,
				RequestHeaders: koboAllowedHeaders(
					r.Header, koboLogRequestHeaders,
				),
				ResponseHeaders: holder.respHeaders,
				UpstreamHeaders: holder.upstreamHeaders,
			})
		}
	}
}

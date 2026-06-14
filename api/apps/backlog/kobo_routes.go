package backlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog/internal/models"
)

// koboRoutes mounts the Kobo native sync protocol endpoints under
// /{prefix}/kobo/. Auth is bearer-token only — AppAccess is NOT used.
//
// Endpoints we own are registered with explicit method+path patterns so they
// take precedence. The trailing-slash catch-all proxies everything else to the
// real Kobo store so firmware updates, store purchases, and account endpoints
// continue to work (proxy mode).
func (app *Backlog) koboRoutes(prefix string, mux *http.ServeMux) {
	base := "/" + prefix + "/kobo"
	mux.HandleFunc("POST "+base+"/v1/initialization", app.koboInitHandler)
	mux.HandleFunc("GET "+base+"/v1/library/sync", app.koboLibrarySyncHandler)
	mux.HandleFunc(
		"GET "+base+"/v1/library/{revisionId}/file", app.koboFileHandler,
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/{revisionId}/state", app.koboGetStateHandler,
	)
	mux.HandleFunc(
		"PUT "+base+"/v1/library/{revisionId}/state", app.koboPutStateHandler,
	)
	// Catch-all: proxy unrecognised paths to the upstream Kobo store.
	mux.HandleFunc(base+"/", app.koboProxyHandler)
}

// koboAuth validates HTTPS and the bearer token.
// Returns (userID, true) on success; writes an error response and returns
// ("", false) on failure — callers must return immediately on false.
func (app *Backlog) koboAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	if proto != "https" {
		http.Error(w, "https required", http.StatusForbidden)
		return "", false
	}

	auth := r.Header.Get("Authorization")
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(auth, bearerPrefix) || len(auth) == len(bearerPrefix) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	raw := auth[len(bearerPrefix):]

	// Always hash — keeps the lookup constant-time-ish regardless of match.
	h := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(h[:])

	userID, err := app.Services.Integrations.GetUserIDByKoboTokenHash(r.Context(), hash)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return "", false
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return "", false
	}
	return userID, true
}

// koboProgressScale converts between Kobo's 0.0–1.0 and our 0–100 integer.
const koboProgressScale = 100

// koboWriteJSON writes v as JSON with status 200.
func koboWriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// --- JSON types for the Kobo store sync protocol ---

type koboInitResponse struct {
	Resources json.RawMessage `json:"Resources"`
	Settings  koboSettings    `json:"Settings"`
	TokenList []string        `json:"TokenList"`
}

type koboSettings struct {
	SynchronizationDelay int    `json:"SynchronizationDelay"`
	TestEmailAddress     string `json:"TestEmailAddress"`
	UserAgent            string `json:"UserAgent"`
}

type koboSyncEntry struct {
	BookEntitlement koboBookEntitlement `json:"BookEntitlement"`
	BookMetadata    koboBookMetadata    `json:"BookMetadata"`
	DownloadUrls    []koboDownloadURL   `json:"DownloadUrls"`
	ReadingState    *koboReadingState   `json:"ReadingState"`
}

type koboBookEntitlement struct {
	Accessibility string            `json:"Accessibility"`
	ActivePeriod  map[string]string `json:"ActivePeriod"`
	Created       string            `json:"Created"`
	//nolint:revive,stylecheck // Kobo protocol field name
	CrossRevisionId string `json:"CrossRevisionId"`
	//nolint:revive,stylecheck // Kobo protocol field name
	Id             string `json:"Id"`
	IsRemoved      bool   `json:"IsRemoved"`
	IsHiddenFromUI bool   `json:"IsHiddenFromUI"`
	PurchasedDate  string `json:"PurchasedDate"`
	//nolint:revive,stylecheck // Kobo protocol field name
	RevisionId string `json:"RevisionId"`
	Status     string `json:"Status"`
	Type       string `json:"Type"`
}

type koboBookMetadata struct {
	Title       string `json:"Title"`
	ContentType string `json:"ContentType"`
	//nolint:revive,stylecheck // Kobo protocol field name
	RevisionId string `json:"RevisionId"`
	Language   string `json:"Language"`
}

type koboDownloadURL struct {
	Format   string `json:"Format"`
	Size     int64  `json:"Size"`
	URL      string `json:"Url"`
	Platform string `json:"Platform"`
}

type koboReadingState struct {
	CurrentBookmark koboBookmark `json:"CurrentBookmark"`
	//nolint:revive,stylecheck // Kobo protocol field name
	EntitlementId string         `json:"EntitlementId"`
	LastModified  string         `json:"LastModified"`
	StatusInfo    koboStatusInfo `json:"StatusInfo"`
}

type koboBookmark struct {
	ContentSourceProgressPercent float64 `json:"ContentSourceProgressPercent"`
	Location                     *string `json:"Location,omitempty"`
}

type koboStatusInfo struct {
	LastModified string `json:"LastModified"`
	Status       string `json:"Status"`
	//nolint:revive,stylecheck // Kobo protocol field name
	TimestampId string `json:"TimestampId"`
}

// --- Handlers ---

// koboInitHandler handles POST /v1/initialization — the device registration
// handshake. We validate the token and return minimal init metadata.
func (app *Backlog) koboInitHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.koboAuth(w, r); !ok {
		return
	}
	koboWriteJSON(w, koboInitResponse{
		Resources: json.RawMessage(`{}`),
		Settings: koboSettings{
			SynchronizationDelay: 0,
			TestEmailAddress:     "",
			UserAgent:            "Kobo",
		},
		TokenList: []string{"BookEntitlement", "BookMetadata", "BookReadingState"},
	})
}

// koboLibrarySyncHandler handles GET /v1/library/sync — merges the upstream
// Kobo store's entitlements with our own kobo-sync books (additive).
func (app *Backlog) koboLibrarySyncHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	books, err := app.Services.Books.ListKoboSyncBooks(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	libraryBase := koboLibraryBase(r)
	now := time.Now().UTC().Format(time.RFC3339)

	ourEntries := make([]json.RawMessage, len(books))
	for i, b := range books {
		id := b.BookID.String()

		// Drive the content type and download format off the stored file format.
		downloadFormat := "KEPUB"
		contentType := "application/x-kobo-epub+zip"
		if b.Format == models.FileFormatPDF {
			downloadFormat = "PDF"
			contentType = "application/pdf"
		}

		// json.Marshal cannot fail on this fully-typed struct.
		raw, _ := json.Marshal(koboSyncEntry{
			BookEntitlement: koboBookEntitlement{
				Accessibility:   "Full",
				ActivePeriod:    map[string]string{"From": now},
				Created:         now,
				CrossRevisionId: id,
				Id:              id,
				IsRemoved:       false,
				IsHiddenFromUI:  false,
				PurchasedDate:   now,
				RevisionId:      id,
				Status:          "Active",
				Type:            "ebook",
			},
			BookMetadata: koboBookMetadata{
				Title:       b.Title,
				ContentType: contentType,
				RevisionId:  id,
				Language:    "en",
			},
			DownloadUrls: []koboDownloadURL{{
				Format:   downloadFormat,
				Size:     b.Size,
				URL:      libraryBase + "/" + id + "/file",
				Platform: "Desktop",
			}},
			ReadingState: nil,
		})
		ourEntries[i] = raw
	}

	// Fetch upstream items (gracefully degrade to empty on error).
	upstreamItems, upstreamHdrs := app.koboFetchUpstreamSync(r)

	// Preserve sync continuation headers the firmware expects.
	for _, hdr := range []string{"x-kobo-sync", "x-kobo-sync-token"} {
		if v := upstreamHdrs.Get(hdr); v != "" {
			w.Header().Set(hdr, v)
		}
	}

	// Upstream items first, then ours (additive — never drops store items).
	all := append(upstreamItems, ourEntries...) //nolint:gocritic // intentional
	koboWriteJSON(w, all)
}

// koboProxyHandler is the catch-all for paths we don't own: it proxies the
// request verbatim to the upstream Kobo store after authenticating our token.
func (app *Backlog) koboProxyHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.koboAuth(w, r); !ok {
		return
	}

	// Strip "/{prefix}/kobo" to obtain the Kobo-relative path.
	_, koboPath, _ := strings.Cut(r.URL.Path, "/kobo")

	targetURL := app.clients.KoboStoreBaseURL + koboPath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	//nolint:gosec // targetURL is built from KoboStoreBaseURL, set by config
	proxyReq, err := http.NewRequestWithContext(
		r.Context(), r.Method, targetURL, r.Body,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	proxyReq.Header = r.Header.Clone()

	//nolint:gosec // intentional proxy to upstream Kobo store
	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// koboFetchUpstreamSync calls the upstream store's /v1/library/sync,
// forwarding all original headers. Returns (items, responseHeaders).
// On any error the items slice is nil (caller degrades gracefully).
func (app *Backlog) koboFetchUpstreamSync(
	r *http.Request,
) ([]json.RawMessage, http.Header) {
	targetURL := app.clients.KoboStoreBaseURL + "/v1/library/sync"
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	//nolint:gosec // targetURL is built from KoboStoreBaseURL, set by config
	req, err := http.NewRequestWithContext(
		r.Context(), http.MethodGet, targetURL, nil,
	)
	if err != nil {
		return nil, nil
	}
	req.Header = r.Header.Clone()

	//nolint:gosec // intentional call to upstream Kobo store
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()

	hdrs := resp.Header.Clone()
	if resp.StatusCode != http.StatusOK {
		return nil, hdrs
	}

	var items []json.RawMessage
	if decErr := json.NewDecoder(resp.Body).Decode(&items); decErr != nil {
		return nil, hdrs
	}
	return items, hdrs
}

// koboFileHandler handles GET /v1/library/{revisionId}/file — issues a 302
// redirect to a short-lived R2 presigned URL for the book's Kobo file.
// The format served (KEPUB or PDF) is determined by the user's per-book
// kobo-format-pdf tag: PDF when present, KEPUB otherwise.
func (app *Backlog) koboFileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, err := uuid.Parse(r.PathValue("revisionId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	format, err := app.Services.Books.GetKoboFileFormat(r.Context(), userID, bookID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	result, err := app.Services.Books.GetBookFile(r.Context(), userID, bookID, format)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	//nolint:gosec // URL comes from internal objectstore presign, not user input
	http.Redirect(w, r, result.URL, http.StatusFound)
}

// koboGetStateHandler handles GET /v1/library/{revisionId}/state.
func (app *Backlog) koboGetStateHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, err := uuid.Parse(r.PathValue("revisionId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	state, err := app.Services.Books.GetReadingState(r.Context(), userID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	koboWriteJSON(w, buildKoboState(bookID.String(), state))
}

// koboPutStateHandler handles PUT /v1/library/{revisionId}/state.
func (app *Backlog) koboPutStateHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, err := uuid.Parse(r.PathValue("revisionId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	var body struct {
		ReadingState struct {
			CurrentBookmark struct {
				ContentSourceProgressPercent float64 `json:"ContentSourceProgressPercent"`
				Location                     *string `json:"Location"`
			} `json:"CurrentBookmark"`
		} `json:"ReadingState"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	pct := int(
		math.Round(
			body.ReadingState.CurrentBookmark.ContentSourceProgressPercent *
				koboProgressScale,
		),
	)
	loc := body.ReadingState.CurrentBookmark.Location

	if err = app.Services.Books.UpdateReadingProgress(
		r.Context(), userID, bookID, models.ReadingSourceKobo, pct, loc,
	); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	state, err := app.Services.Books.GetReadingState(r.Context(), userID, bookID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	koboWriteJSON(w, buildKoboState(bookID.String(), state))
}

// --- helpers ---

// koboLibraryBase derives the https://host/…/kobo/v1/library prefix from the
// current request. koboAuth already enforces HTTPS, so the scheme is fixed.
func koboLibraryBase(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	// r.URL.Path is "/{prefix}/kobo/v1/library/sync" — strip "/sync" suffix.
	base := strings.TrimSuffix(r.URL.Path, "/sync")
	return "https://" + host + base
}

// buildKoboState converts an optional BookReadingState into the Kobo reading
// state JSON shape returned by GET and PUT state endpoints.
func buildKoboState(id string, state *models.BookReadingState) *koboReadingState {
	if state == nil {
		return &koboReadingState{
			CurrentBookmark: koboBookmark{
				ContentSourceProgressPercent: 0,
				Location:                     nil,
			},
			EntitlementId: id,
			LastModified:  time.Now().UTC().Format(time.RFC3339),
			StatusInfo: koboStatusInfo{
				LastModified: time.Now().UTC().Format(time.RFC3339),
				Status:       "ReadyToRead",
				TimestampId:  id,
			},
		}
	}
	ts := state.UpdatedAt.UTC().Format(time.RFC3339)
	return &koboReadingState{
		CurrentBookmark: koboBookmark{
			ContentSourceProgressPercent: float64(state.Percent) / koboProgressScale,
			Location:                     state.Location,
		},
		EntitlementId: id,
		LastModified:  ts,
		StatusInfo: koboStatusInfo{
			LastModified: ts,
			Status:       "ReadyToRead",
			TimestampId:  id,
		},
	}
}

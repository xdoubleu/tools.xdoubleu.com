package books

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// koboUpstreamTimeout bounds calls to the upstream Kobo store so a slow or
// stalled upstream can never hang a device's sync request indefinitely.
// r.Context() still cancels on device disconnect; Timeout adds an absolute
// ceiling on top of that.
const koboUpstreamTimeout = 10 * time.Second

//nolint:gochecknoglobals // shared client, Timeout mutated only in tests
var koboUpstreamClient = &http.Client{Timeout: koboUpstreamTimeout}

// koboRoutes mounts the Kobo native sync protocol endpoints under
// /{prefix}/kobo/{token}/. The token is a raw bearer secret embedded in the
// device's api_endpoint URL by the web setup flow; it is SHA-256 hashed before
// the DB lookup so the plaintext is never stored. AppAccess is NOT used.
//
// The device's firmware sets api_endpoint = <our base>/{token}, then appends
// store-protocol paths (e.g. /v1/initialization, /v1/library/sync). Each
// request therefore arrives as /{prefix}/kobo/{token}/v1/…. We own explicit
// patterns for the endpoints we implement; the catch-all proxies everything
// else (firmware updates, store purchases, auth) to the real Kobo store so
// those continue to work.
func (app *Books) koboRoutes(prefix string, mux *http.ServeMux) {
	base := "/" + prefix + "/kobo/{token}"
	mux.HandleFunc(
		"POST "+base+"/v1/initialization", app.koboLogged(app.koboInitHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/sync", app.koboLogged(app.koboLibrarySyncHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/{revisionId}/file",
		app.koboLogged(app.koboFileHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/{revisionId}/metadata",
		app.koboLogged(app.koboMetadataHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/v1/library/{revisionId}/state",
		app.koboLogged(app.koboGetStateHandler),
	)
	mux.HandleFunc(
		"PUT "+base+"/v1/library/{revisionId}/state",
		app.koboLogged(app.koboPutStateHandler),
	)
	// Catch-all: proxy unrecognised paths to the upstream Kobo store.
	mux.HandleFunc(
		"/"+prefix+"/kobo/{token}/", app.koboLogged(app.koboProxyHandler),
	)
}

// koboAuth validates HTTPS and the token embedded in the request URL path.
// Returns (userID, true) on success; writes an error response and returns
// ("", false) on failure — callers must return immediately on false.
func (app *Books) koboAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	if proto != "https" {
		http.Error(w, "https required", http.StatusForbidden)
		return "", false
	}

	raw := r.PathValue("token")
	if raw == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}

	// Always hash — keeps the lookup constant-time-ish regardless of match.
	h := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(h[:])

	userID, deviceID, err := app.Services.Kobo.GetKoboAuthByTokenHash(
		r.Context(), hash,
	)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return "", false
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return "", false
	}

	// Arm request/response capture for this device when debug logging is on.
	// koboAuth runs before any handler reads the body or writes a response, so
	// the capture layer sees the flag in time.
	if holder := koboLogHolderFrom(r.Context()); holder != nil {
		holder.deviceID = deviceID
		holder.enabled = app.Services.KoboLog.IsEnabled(deviceID)
	}
	return userID, true
}

// koboEpoch is used as LastModified when no reading state exists server-side.
// Returning time.Now() would make the server always appear newer than the
// device, causing the firmware to overwrite local progress with the server's
// 0% and never issue PUT …/state. An epoch timestamp ensures the device's
// local progress is always "newer" so it wins the conflict and pushes its
// progress to us.
const koboEpoch = "1970-01-01T00:00:00Z"

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
	ReadingState    *koboReadingState   `json:"ReadingState"`
}

// koboNewEntitlement wraps a koboSyncEntry in the discriminator key that the
// Kobo firmware requires. Each element of the /v1/library/sync array must be
// an object keyed by a change-type tag (NewEntitlement, ChangedEntitlement,
// etc.) with the payload nested inside — a bare payload is silently ignored.
type koboNewEntitlement struct {
	NewEntitlement koboSyncEntry `json:"NewEntitlement"`
}

// koboChangedEntitlement is the discriminator used for an existing
// entitlement whose state changed — we use it to signal a removal
// (BookEntitlement.IsRemoved: true) for a book that was previously synced.
type koboChangedEntitlement struct {
	ChangedEntitlement koboSyncEntry `json:"ChangedEntitlement"`
}

type koboBookEntitlement struct {
	Accessibility string            `json:"Accessibility"`
	ActivePeriod  map[string]string `json:"ActivePeriod"`
	Created       string            `json:"Created"`
	//nolint:revive // Kobo protocol field name
	CrossRevisionId string `json:"CrossRevisionId"`
	//nolint:revive // Kobo protocol field name
	Id             string `json:"Id"`
	IsRemoved      bool   `json:"IsRemoved"`
	IsHiddenFromUI bool   `json:"IsHiddenFromUI"`
	PurchasedDate  string `json:"PurchasedDate"`
	//nolint:revive // Kobo protocol field name
	RevisionId string `json:"RevisionId"`
	Status     string `json:"Status"`
	Type       string `json:"Type"`
}

type koboBookMetadata struct {
	Title       string `json:"Title"`
	ContentType string `json:"ContentType"`
	//nolint:revive // Kobo protocol field name
	RevisionId   string            `json:"RevisionId"`
	Language     string            `json:"Language"`
	DownloadUrls []koboDownloadURL `json:"DownloadUrls"`
}

type koboDownloadURL struct {
	Format   string `json:"Format"`
	Size     int64  `json:"Size"`
	URL      string `json:"Url"`
	Platform string `json:"Platform"`
}

type koboReadingState struct {
	CurrentBookmark koboBookmark `json:"CurrentBookmark"`
	//nolint:revive // Kobo protocol field name
	EntitlementId string         `json:"EntitlementId"`
	LastModified  string         `json:"LastModified"`
	StatusInfo    koboStatusInfo `json:"StatusInfo"`
}

type koboBookmark struct {
	ProgressPercent int `json:"ProgressPercent"`
	// ContentSourceProgressPercent is the within-chapter position on real
	// devices; we don't track that granularity, so we mirror the whole-book
	// ProgressPercent here too — good enough for the firmware's progress bar.
	ContentSourceProgressPercent int     `json:"ContentSourceProgressPercent"`
	Location                     *string `json:"Location,omitempty"`
}

type koboStatusInfo struct {
	LastModified string `json:"LastModified"`
	Status       string `json:"Status"`
	//nolint:revive // Kobo protocol field name
	TimestampId string `json:"TimestampId"`
}

// --- Handlers ---

// koboInitHandler handles POST /v1/initialization — the device registration
// handshake. We validate the token and return minimal init metadata.
func (app *Books) koboInitHandler(w http.ResponseWriter, r *http.Request) {
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
func (app *Books) koboLibrarySyncHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	books, err := app.Services.Books.ListKoboSyncBooks(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Batch-load all reading states so the sync manifest can include them
	// without issuing a per-book query (avoids N+1).
	stateByBook, err := app.Services.Books.ListReadingStates(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	libraryBase := app.koboLibraryBase(r)

	ourEntries := make([]json.RawMessage, len(books))
	for i, b := range books {
		id := b.BookID.String()
		// Use the time kobo-sync was enabled for this book so the entitlement
		// payload is byte-identical on every sync. time.Now() would produce a
		// different Created/PurchasedDate each request, causing the Kobo firmware
		// to tear down and recreate the entitlement on every sync (the visible
		// "books briefly disappear" flicker).
		enabled := b.KoboSyncEnabledAt.UTC().Format(time.RFC3339)

		// json.Marshal cannot fail on this fully-typed struct.
		// Each entry must be wrapped in the NewEntitlement discriminator key
		// so the Kobo firmware recognises it — a bare payload is silently ignored.
		// DownloadUrls lives inside BookMetadata per the Kobo store protocol.
		// ReadingState must be non-nil so the firmware participates in
		// reading-state sync and issues PUT …/state on progress changes.
		raw, _ := json.Marshal(koboNewEntitlement{
			NewEntitlement: koboSyncEntry{
				BookEntitlement: koboBookEntitlement{
					Accessibility:   "Full",
					ActivePeriod:    map[string]string{"From": enabled},
					Created:         enabled,
					CrossRevisionId: id,
					Id:              id,
					IsRemoved:       false,
					IsHiddenFromUI:  false,
					PurchasedDate:   enabled,
					RevisionId:      id,
					Status:          "Active",
					Type:            "ebook",
				},
				BookMetadata: buildKoboMetadata(b, libraryBase),
				ReadingState: buildKoboState(id, stateByBook[b.BookID]),
			},
		})
		ourEntries[i] = raw
	}

	removals, err := app.Services.Books.ListKoboRemovals(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	removalEntries := make([]json.RawMessage, len(removals))
	for i, rm := range removals {
		removalEntries[i] = buildKoboRemovalEntry(rm)
	}

	// Fetch upstream items (gracefully degrade to empty on error).
	upstreamItems, upstreamHdrs := app.koboFetchUpstreamSync(r)

	// Preserve sync continuation headers the firmware expects.
	for _, hdr := range []string{"x-kobo-sync", "x-kobo-sync-token"} {
		if v := upstreamHdrs.Get(hdr); v != "" {
			w.Header().Set(hdr, v)
		}
	}

	// Upstream items first, then ours, then removals (additive — never drops
	// store items).
	all := append(upstreamItems, ourEntries...) //nolint:gocritic // intentional
	all = append(all, removalEntries...)
	if all == nil {
		// A nil slice would encode as JSON null, which the Kobo firmware doesn't
		// treat as "sync complete" — it hangs at "Checking for updates…". Happens
		// when there are zero kobo-sync books and upstream sync is unavailable.
		all = []json.RawMessage{}
	}
	koboWriteJSON(w, all)
}

// koboProxyHandler is the catch-all for paths we don't own: it proxies the
// request verbatim to the upstream Kobo store after authenticating our token.
// The token segment is stripped so the upstream receives a clean /v1/… path.
func (app *Books) koboProxyHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.koboAuth(w, r); !ok {
		return
	}

	// Strip "/{prefix}/kobo/{token}" to obtain the Kobo-relative path.
	token := r.PathValue("token")
	_, koboPath, _ := strings.Cut(r.URL.Path, "/kobo/"+token)

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
	resp, err := koboUpstreamClient.Do(proxyReq)
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
func (app *Books) koboFetchUpstreamSync(
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
	resp, err := koboUpstreamClient.Do(req)
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

// buildKoboMetadata constructs the BookMetadata payload for a kobo-sync book.
// It is used by both the library sync handler and the dedicated metadata
// endpoint so the two responses stay byte-identical (the device cross-checks).
func buildKoboMetadata(b models.KoboSyncBook, libraryBase string) koboBookMetadata {
	downloadFormat := "KEPUB"
	contentType := "application/x-kobo-epub+zip"
	if b.Format == models.FileFormatPDF {
		downloadFormat = "PDF"
		contentType = "application/pdf"
	}
	id := b.BookID.String()
	return koboBookMetadata{
		Title:       b.Title,
		ContentType: contentType,
		RevisionId:  id,
		Language:    "en",
		DownloadUrls: []koboDownloadURL{{
			Format:   downloadFormat,
			Size:     b.Size,
			URL:      libraryBase + "/" + id + "/file",
			Platform: "Generic",
		}},
	}
}

// buildKoboRemovalEntry builds a ChangedEntitlement payload telling the
// device to delete a book it previously synced. We only have the book ID and
// the tombstone timestamp (the catalog row may already be gone), so
// BookMetadata is left minimal.
//
// ponytail: the exact discriminator/shape the firmware needs for a removal
// (ChangedEntitlement vs NewEntitlement, whether it tolerates empty
// BookMetadata) isn't verifiable without a real device — confirm on-device
// and adjust if the book isn't actually removed.
func buildKoboRemovalEntry(rm models.KoboRemoval) json.RawMessage {
	id := rm.BookID.String()
	removed := rm.RemovedAt.UTC().Format(time.RFC3339)
	// json.Marshal cannot fail on this fully-typed struct.
	raw, _ := json.Marshal(koboChangedEntitlement{
		ChangedEntitlement: koboSyncEntry{
			BookEntitlement: koboBookEntitlement{
				Accessibility:   "Full",
				ActivePeriod:    map[string]string{},
				Created:         removed,
				CrossRevisionId: id,
				Id:              id,
				IsRemoved:       true,
				IsHiddenFromUI:  true,
				PurchasedDate:   removed,
				RevisionId:      id,
				Status:          "Active",
				Type:            "ebook",
			},
			BookMetadata: koboBookMetadata{ //nolint:exhaustruct // removal: no
				// download/content details to give — the book is gone.
				RevisionId: id,
			},
			ReadingState: nil,
		},
	})
	return raw
}

// koboMetadataHandler handles GET /v1/library/{revisionId}/metadata.
// If the book belongs to the authenticated user's kobo-sync list it is served
// locally; otherwise the request is proxied to the upstream Kobo store so
// genuine store purchases keep working (same additive philosophy as the sync
// handler).
func (app *Books) koboMetadataHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, err := uuid.Parse(r.PathValue("revisionId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	book, err := app.Services.Books.GetKoboSyncBook(r.Context(), userID, bookID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			// Not one of our kobo-sync books — proxy to the upstream store.
			app.koboProxyHandler(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	meta := buildKoboMetadata(book, app.koboLibraryBase(r))
	koboWriteJSON(w, []koboBookMetadata{meta})
}

// koboFileHandler handles GET /v1/library/{revisionId}/file — issues a 302
// redirect to a short-lived R2 presigned URL for the book's Kobo file.
// The format served (KEPUB or PDF) is determined by the user's per-book
// kobo-format-pdf tag: PDF when present, KEPUB otherwise.
func (app *Books) koboFileHandler(w http.ResponseWriter, r *http.Request) {
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

	http.Redirect(w, r, result.URL, http.StatusFound)
}

// koboGetStateHandler handles GET /v1/library/{revisionId}/state.
func (app *Books) koboGetStateHandler(w http.ResponseWriter, r *http.Request) {
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
func (app *Books) koboPutStateHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, err := uuid.Parse(r.PathValue("revisionId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	// The device sends a plural "ReadingStates" array (not a singular
	// "ReadingState" object), whole-book progress as an integer 0-100 in
	// ProgressPercent (ContentSourceProgressPercent is the within-chapter
	// position, which we don't track), and Location as a
	// {Source,Type,Value} object rather than a bare string.
	var body struct {
		ReadingStates []struct {
			CurrentBookmark struct {
				ProgressPercent int             `json:"ProgressPercent"`
				Location        json.RawMessage `json:"Location"`
			} `json:"CurrentBookmark"`
		} `json:"ReadingStates"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// An empty array carries no update — leave existing progress untouched.
	if len(body.ReadingStates) > 0 {
		bm := body.ReadingStates[len(body.ReadingStates)-1].CurrentBookmark
		loc := parseKoboLocation(bm.Location)

		if err = app.Services.Books.UpdateReadingProgress(
			r.Context(), userID, bookID, models.ReadingSourceKobo,
			bm.ProgressPercent, loc,
		); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	// No prior state (e.g. an empty ReadingStates PUT on a book never synced
	// before) is not an error — buildKoboState handles nil as 0%/ReadyToRead.
	state, err := app.Services.Books.GetReadingState(r.Context(), userID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	koboWriteJSON(w, buildKoboState(bookID.String(), state))
}

// --- helpers ---

// koboLibraryBase derives the https://host/…/kobo/{token}/v1/library prefix
// used to build per-book file download URLs returned in the sync manifest and
// the metadata endpoint.
//
// When clients.PublicAPIBaseURL is set (e.g. "https://tools.xdoubleu.com/api")
// it is used directly. This is necessary when a reverse proxy strips a path
// prefix (e.g. /api) before forwarding to this server, because r.URL.Path
// would not contain that prefix. koboAuth already enforces HTTPS for the
// device-facing request, so the scheme is fixed to https in both paths.
func (app *Books) koboLibraryBase(r *http.Request) string {
	// Cut the path at /v1/library so this works for any sub-path
	// (e.g. /v1/library/sync, /v1/library/{id}/metadata, etc.).
	path := r.URL.Path
	if idx := strings.Index(path, "/v1/library"); idx != -1 {
		path = path[:idx] + "/v1/library"
	}
	if app.clients.PublicAPIBaseURL != "" {
		return strings.TrimSuffix(app.clients.PublicAPIBaseURL, "/") + path
	}
	// Fallback: derive scheme+host from request headers (dev / test).
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return "https://" + host + path
}

// parseKoboLocation extracts a resumable location string from the Kobo
// CurrentBookmark.Location field, which real devices send as an object
// {Source,Type,Value}. Falls back to a bare JSON string for older/other
// clients, else nil.
func parseKoboLocation(raw json.RawMessage) *string {
	if len(raw) == 0 {
		return nil
	}
	var obj struct {
		Value string `json:"Value"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Value != "" {
		return &obj.Value
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return &s
	}
	return nil // ponytail: object with empty Value → no location, acceptable
}

// koboStatusForPercent derives the Kobo StatusInfo.Status the firmware
// expects from our stored percent, since we don't persist a separate status
// column — it's fully determined by progress (0 / partial / complete).
func koboStatusForPercent(percent int) string {
	switch {
	case percent >= models.MaxProgressPercent:
		return "Finished"
	case percent > 0:
		return "Reading"
	default:
		return "ReadyToRead"
	}
}

// buildKoboState converts an optional BookReadingState into the Kobo reading
// state JSON shape returned by GET and PUT state endpoints.
func buildKoboState(id string, state *models.BookReadingState) *koboReadingState {
	if state == nil {
		return &koboReadingState{
			CurrentBookmark: koboBookmark{
				ProgressPercent:              0,
				ContentSourceProgressPercent: 0,
				Location:                     nil,
			},
			EntitlementId: id,
			LastModified:  koboEpoch,
			StatusInfo: koboStatusInfo{
				LastModified: koboEpoch,
				Status:       "ReadyToRead",
				TimestampId:  id,
			},
		}
	}
	ts := state.UpdatedAt.UTC().Format(time.RFC3339)
	return &koboReadingState{
		CurrentBookmark: koboBookmark{
			ProgressPercent:              state.Percent,
			ContentSourceProgressPercent: state.Percent,
			Location:                     state.Location,
		},
		EntitlementId: id,
		LastModified:  ts,
		StatusInfo: koboStatusInfo{
			LastModified: ts,
			Status:       koboStatusForPercent(state.Percent),
			TimestampId:  id,
		},
	}
}

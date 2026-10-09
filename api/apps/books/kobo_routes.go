package books

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/internal/database"
)

// koboUpstreamTimeout caps upstream Kobo store calls so a stalled upstream
// never hangs a device's sync.
const koboUpstreamTimeout = 10 * time.Second

//nolint:gochecknoglobals // shared client, Timeout mutated only in tests
var koboUpstreamClient = &http.Client{Timeout: koboUpstreamTimeout}

// koboRoutes mounts the Kobo sync protocol under /{prefix}/kobo/{token}/. The
// token is a bearer secret in the device's api_endpoint, SHA-256 hashed before
// lookup; AppAccess is not used. Unhandled paths are proxied to the real Kobo
// store so firmware updates, purchases and auth keep working.
func (app *Books) koboRoutes(prefix string, mux *http.ServeMux) {
	base := "/" + prefix + "/kobo/{token}"
	// The device fetches initialization with GET; serving POST too is harmless.
	mux.HandleFunc(
		"GET "+base+"/v1/initialization", app.koboLogged(app.koboInitHandler),
	)
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
		"GET "+base+"/{revisionId}/{width}/{height}/{greyscale}/image.jpg",
		app.koboLogged(app.koboCoverHandler),
	)
	mux.HandleFunc(
		"GET "+base+"/{revisionId}/{width}/{height}/{quality}/{greyscale}/image.jpg",
		app.koboLogged(app.koboCoverHandler),
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
	mux.HandleFunc(
		"/"+prefix+"/kobo/{token}/", app.koboLogged(app.koboProxyHandler),
	)
}

// koboAuth validates HTTPS and the URL token. On false it has already written
// the error response.
func (app *Books) koboAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, _, ok := app.koboAuthDevice(w, r)
	return userID, ok
}

// koboAuthDevice is koboAuth that also returns the calling device's ID.
func (app *Books) koboAuthDevice(
	w http.ResponseWriter,
	r *http.Request,
) (string, string, bool) {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	if proto != "https" {
		http.Error(w, "https required", http.StatusForbidden)
		return "", "", false
	}

	raw := r.PathValue("token")
	if raw == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", "", false
	}

	h := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(h[:])

	userID, deviceID, err := app.Services.Kobo.GetKoboAuthByTokenHash(
		r.Context(), hash,
	)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return "", "", false
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return "", "", false
	}

	// Arm debug capture here: this runs before any handler touches the body.
	if holder := koboLogHolderFrom(r.Context()); holder != nil {
		holder.deviceID = deviceID
		holder.enabled = app.Services.KoboLog.IsEnabled(deviceID)
	}
	return userID, deviceID, true
}

// koboEpoch is LastModified when no server state exists. time.Now() would make
// the firmware overwrite local progress with 0% and never PUT state.
const koboEpoch = "1970-01-01T00:00:00Z"

func koboWriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

type koboInitResponse struct {
	Resources koboResources `json:"Resources"`
	Settings  koboSettings  `json:"Settings"`
	TokenList []string      `json:"TokenList"`
}

// koboResources advertises the cover image templates and library endpoints
// the device uses during sync. The library_* keys are required for the
// firmware to advance past initialization to /v1/library/sync; omitting them
// stalls the sync (see Calibre-Web's HandleInitRequest).
type koboResources struct {
	ImageHost string `json:"image_host"`

	ImageURLQualityTemplate string `json:"image_url_quality_template"`

	ImageURLTemplate string `json:"image_url_template"`

	LibrarySync string `json:"library_sync"`

	LibraryMetadata string `json:"library_metadata"`

	ReadingState string `json:"reading_state"`
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

// koboNewEntitlement wraps an entry in the change-type key the firmware
// requires; a bare payload is silently ignored.
type koboNewEntitlement struct {
	NewEntitlement koboSyncEntry `json:"NewEntitlement"`
}

// koboChangedEntitlement signals a removal of a previously synced book.
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
	// CoverImageId names the image the device requests from
	// image_url(_quality)_template; the book's own UUID, like Id.
	CoverImageID string `json:"CoverImageId"`
}

type koboDownloadURL struct {
	Format   string `json:"Format"`
	Size     int64  `json:"Size"`
	URL      string `json:"Url"`
	Platform string `json:"Platform"`
}

// koboInitHandler handles POST /v1/initialization.
func (app *Books) koboInitHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.koboAuth(w, r); !ok {
		return
	}
	// The x-kobo-apitoken header proves the store handshake to the firmware;
	// without it the device treats init as incomplete and never advances to
	// /v1/library/sync. Set before koboWriteJSON writes the response.
	w.Header().Set("x-kobo-apitoken", "e30=")
	coverBase := app.koboCoverBase(r)
	// For init the request path is /v1/initialization (not /v1/library/...), so
	// koboLibraryBase can't strip it; derive the library prefix from coverBase.
	libraryBase := coverBase + "/v1/library"
	koboWriteJSON(w, koboInitResponse{
		Resources: koboResources{
			ImageHost: coverBase,
			ImageURLQualityTemplate: coverBase +
				"/{ImageId}/{width}/{height}/{Quality}/isGreyscale/image.jpg",
			ImageURLTemplate: coverBase +
				"/{ImageId}/{width}/{height}/false/image.jpg",
			LibrarySync:     libraryBase + "/sync",
			LibraryMetadata: libraryBase + "/{Ids}/metadata",
			ReadingState:    libraryBase + "/{Ids}/state",
		},
		Settings: koboSettings{
			SynchronizationDelay: 0,
			TestEmailAddress:     "",
			UserAgent:            "Kobo",
		},
		TokenList: []string{"BookEntitlement", "BookMetadata", "BookReadingState"},
	})
}

// koboCoverBase derives the https://…/kobo/{token} prefix used by the cover
// image templates, by stripping the initialization path from the request. Same
// origin rules as koboLibraryBase.
func (app *Books) koboCoverBase(r *http.Request) string {
	path := r.URL.Path
	if idx := strings.Index(path, "/v1/initialization"); idx != -1 {
		path = path[:idx]
	}
	if app.clients.PublicAPIBaseURL != "" {
		return strings.TrimSuffix(app.clients.PublicAPIBaseURL, "/") + path
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return "https://" + host + path
}

// koboLibrarySyncHandler handles GET /v1/library/sync, adding our kobo-sync
// books to the upstream store's entitlements.
func (app *Books) koboLibrarySyncHandler(w http.ResponseWriter, r *http.Request) {
	userID, deviceID, ok := app.koboAuthDevice(w, r)
	if !ok {
		return
	}

	books, err := app.Services.Books.ListKoboSyncBooks(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	stateByBook, err := app.Services.Books.ListReadingStates(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// The upstream fetch overlaps the span translation's wait.
	var upstream koboUpstream
	upstreamDone := make(chan struct{})
	go func() {
		defer close(upstreamDone)
		upstream = app.koboFetchUpstreamSync(r)
	}()

	held, pending, err := app.koboSyncSpans(
		r.Context(), userID, deviceID, books, stateByBook,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	libraryBase := app.koboLibraryBase(r)

	ourEntries := make([]json.RawMessage, len(books))
	for i, b := range books {
		ourEntries[i] = app.buildKoboSyncEntry(r, userID, b, stateByBook, libraryBase)
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

	<-upstreamDone

	if upstream.note != "" {
		koboSetUpstreamNote(r, w, upstream.note)
		app.Logger.Warn("kobo upstream store sync failed",
			"device_id", deviceID, "note", upstream.note)
	}

	// Last before responding: this marks the states as delivered.
	stateEntries, err := app.koboChangedReadingStates(
		r.Context(), deviceID, books, stateByBook, held, pending,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	for _, hdr := range []string{"x-kobo-sync", "x-kobo-sync-token"} {
		if v := upstream.hdrs.Get(hdr); v != "" {
			w.Header().Set(hdr, v)
		}
	}

	all := append(upstream.items, ourEntries...) //nolint:gocritic // intentional
	all = append(all, stateEntries...)
	all = append(all, removalEntries...)
	if all == nil {
		// A nil slice encodes as null, which hangs the firmware at "Checking for
		// updates…".
		all = []json.RawMessage{}
	}
	koboWriteJSON(w, all)
}

// buildKoboSyncEntry builds one book's entitlement. RevisionId stays the bare
// book UUID: the firmware keys on Id and a varying RevisionId adds a second
// copy. A regenerated KEPUB (ConverterVersion differs from
// LastSyncedConverterVersion) is sent as ChangedEntitlement so the device
// invalidates its download instead of duplicating it.
func (app *Books) buildKoboSyncEntry(
	r *http.Request,
	userID string,
	b models.KoboSyncBook,
	stateByBook map[uuid.UUID]*models.BookReadingState,
	libraryBase string,
) json.RawMessage {
	id := b.BookID.String()
	// KoboSyncEnabledAt keeps the payload byte-identical across syncs; time.Now()
	// makes the firmware recreate the entitlement each time (books flicker).
	enabled := b.KoboSyncEnabledAt.UTC().Format(time.RFC3339)
	isReplace := koboIsReplace(b)

	if b.Format == models.FileFormatKEPUB &&
		app.Services.Conversion.IsKEPUBStale(b.ConverterVersion) {
		app.startKEPUBRegeneration(r.Context(), userID, b.BookID)
	}

	entry := koboSyncEntry{
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
	}

	// ReadingState must be non-nil so the firmware PUTs state on progress.
	var raw []byte
	if isReplace {
		raw, _ = json.Marshal(koboChangedEntitlement{ChangedEntitlement: entry})
	} else {
		raw, _ = json.Marshal(koboNewEntitlement{NewEntitlement: entry})
	}

	if b.LastSyncedConverterVersion == nil ||
		*b.LastSyncedConverterVersion != b.ConverterVersion {
		if updErr := app.Services.Books.UpdateKoboLastSyncedConverterVersion(
			r.Context(), userID, b.BookID, b.ConverterVersion,
		); updErr != nil {
			app.Logger.Error("failed to update kobo last synced converter version",
				"error", updErr, "bookID", b.BookID)
		}
	}

	return raw
}

// koboProxyHandler proxies paths we don't own to the upstream Kobo store,
// stripping the token segment.
func (app *Books) koboProxyHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := app.koboAuth(w, r); !ok {
		return
	}

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
	koboForwardAuth(proxyReq.Header, r.Header)

	//nolint:gosec // intentional proxy to upstream Kobo store
	resp, err := koboUpstreamClient.Do(proxyReq)
	if err != nil {
		koboSetUpstreamNote(r, w, "upstream: "+err.Error())
		app.Logger.Warn("kobo upstream proxy failed", "err", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		koboSetUpstreamNote(r, w, koboUpstreamNote(resp.StatusCode, r.Header))
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// koboUpstream holds a merged-upstream sync result and a diagnostic note about
// how the store call went. note is empty on success; on failure it carries the
// upstream HTTP status (or error) and whether the device's Authorization was
// present to forward — never the credential value.
type koboUpstream struct {
	items []json.RawMessage
	hdrs  http.Header
	note  string
}

// koboForwardAuth copies the device's store credential from the arriving
// request onto the upstream call. Kobo identifies its account to the real
// store through the Authorization header on the device request; with none to
// forward the store answers 401 and the merge silently empties.
func koboForwardAuth(dst, src http.Header) {
	if v := src.Get("Authorization"); v != "" {
		dst.Set("Authorization", v)
	}
}

// koboUpstreamNote is the diagnostic for a failed upstream store call: the
// status and whether an Authorization header was present to forward.
func koboUpstreamNote(status int, src http.Header) string {
	auth := "Authorization forwarded"
	if src.Get("Authorization") == "" {
		auth = "no Authorization forwarded"
	}
	return fmt.Sprintf("upstream %d (%s)", status, auth)
}

// koboFetchUpstreamSync returns upstream sync items (empty on failure) and
// headers, with a diagnostic note for the Kobo debug log when the store call
// fails or is unauthenticated.
func (app *Books) koboFetchUpstreamSync(
	r *http.Request,
) koboUpstream {
	targetURL := app.clients.KoboStoreBaseURL + "/v1/library/sync"
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	//nolint:gosec // targetURL is built from KoboStoreBaseURL, set by config
	req, err := http.NewRequestWithContext(
		r.Context(), http.MethodGet, targetURL, nil,
	)
	if err != nil {
		//nolint:exhaustruct // failure: items/hdrs stay nil
		return koboUpstream{note: "upstream: build request failed"}
	}
	req.Header = r.Header.Clone()
	koboForwardAuth(req.Header, r.Header)

	//nolint:gosec // intentional call to upstream Kobo store
	resp, err := koboUpstreamClient.Do(req)
	if err != nil {
		//nolint:exhaustruct // failure: items/hdrs stay nil
		return koboUpstream{note: "upstream: " + err.Error()}
	}
	defer resp.Body.Close()

	hdrs := resp.Header.Clone()
	if resp.StatusCode != http.StatusOK {
		//nolint:exhaustruct // failure: items stay nil
		return koboUpstream{hdrs: hdrs, note: koboUpstreamNote(resp.StatusCode, r.Header)}
	}

	var items []json.RawMessage
	if decErr := json.NewDecoder(resp.Body).Decode(&items); decErr != nil {
		//nolint:exhaustruct // failure: items stay nil
		return koboUpstream{hdrs: hdrs, note: "upstream: decode failed"}
	}
	return koboUpstream{items: items, hdrs: hdrs, note: ""}
}

// startKEPUBRegeneration regenerates a stale KEPUB in a detached goroutine:
// the sync request must never wait on a PDF conversion (ADR-0017).
func (app *Books) startKEPUBRegeneration(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) {
	convCtx := context.WithoutCancel(ctx)
	go func() {
		_, _ = app.Services.Conversion.EnsureKEPUB(convCtx, userID, bookID)
	}()
}

// buildKoboMetadata builds BookMetadata; shared by sync and the metadata
// endpoint because the device cross-checks them.
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
		CoverImageID: id,
	}
}

// buildKoboRemovalEntry builds a ChangedEntitlement telling the device to
// delete a synced book; the catalog row may be gone, so metadata is minimal.
// Unverified on a real device.
func buildKoboRemovalEntry(rm models.KoboRemoval) json.RawMessage {
	id := rm.BookID.String()
	removed := rm.RemovedAt.UTC().Format(time.RFC3339)
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

// koboMetadataHandler handles GET /v1/library/{revisionId}/metadata, serving
// our kobo-sync books locally and proxying the rest upstream.
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
			app.koboProxyHandler(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	meta := buildKoboMetadata(book, app.koboLibraryBase(r))
	koboWriteJSON(w, []koboBookMetadata{meta})
}

// koboFileHandler handles GET /v1/library/{revisionId}/file with a 302 to a
// presigned R2 URL; PDF when the book has the kobo-format-pdf tag, else KEPUB.
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

// koboCoverHandler serves a kobo-sync book's cached cover as a 302 to a
// presigned R2 URL (mirroring coverHandler), gated to the caller's own books.
func (app *Books) koboCoverHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.koboAuth(w, r)
	if !ok {
		return
	}

	bookID, err := uuid.Parse(r.PathValue("revisionId"))
	if err != nil {
		http.Error(w, "invalid book id", http.StatusBadRequest)
		return
	}

	if _, gErr := app.Services.Books.GetKoboSyncBook(
		r.Context(), userID, bookID,
	); gErr != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), coverCtxTimeout)
	defer cancel()

	result, err := app.Services.Books.GetBookCover(ctx, bookID)
	if err != nil {
		if errors.Is(err, services.ErrCoverNotFound) {
			http.Error(w, "cover not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, result.URL, http.StatusFound)
}

// koboLibraryBase derives the https://…/kobo/{token}/v1/library prefix.
// PublicAPIBaseURL wins when set, since a reverse proxy may strip /api from
// r.URL.Path. The scheme is always https (koboAuth enforces it).
func (app *Books) koboLibraryBase(r *http.Request) string {
	path := r.URL.Path
	if idx := strings.Index(path, "/v1/library"); idx != -1 {
		path = path[:idx] + "/v1/library"
	}
	if app.clients.PublicAPIBaseURL != "" {
		return strings.TrimSuffix(app.clients.PublicAPIBaseURL, "/") + path
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return "https://" + host + path
}

package books

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
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
	mux.HandleFunc(
		"/"+prefix+"/kobo/{token}/", app.koboLogged(app.koboProxyHandler),
	)
}

// koboAuth validates HTTPS and the URL token. On false it has already written
// the error response.
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

	// Arm debug capture here: this runs before any handler touches the body.
	if holder := koboLogHolderFrom(r.Context()); holder != nil {
		holder.deviceID = deviceID
		holder.enabled = app.Services.KoboLog.IsEnabled(deviceID)
	}
	return userID, true
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
	// ContentSourceProgressPercent is within-chapter on devices; we mirror the
	// whole-book percent.
	ContentSourceProgressPercent int     `json:"ContentSourceProgressPercent"`
	Location                     *string `json:"Location,omitempty"`
}

type koboStatusInfo struct {
	LastModified string `json:"LastModified"`
	Status       string `json:"Status"`
	//nolint:revive // Kobo protocol field name
	TimestampId string `json:"TimestampId"`
}

// koboInitHandler handles POST /v1/initialization.
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

// koboLibrarySyncHandler handles GET /v1/library/sync, adding our kobo-sync
// books to the upstream store's entitlements.
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

	stateByBook, err := app.Services.Books.ListReadingStates(r.Context(), userID)
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

	upstreamItems, upstreamHdrs := app.koboFetchUpstreamSync(r)

	for _, hdr := range []string{"x-kobo-sync", "x-kobo-sync-token"} {
		if v := upstreamHdrs.Get(hdr); v != "" {
			w.Header().Set(hdr, v)
		}
	}

	all := append(upstreamItems, ourEntries...) //nolint:gocritic // intentional
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
	isReplace := b.LastSyncedConverterVersion != nil &&
		*b.LastSyncedConverterVersion != b.ConverterVersion

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

// koboFetchUpstreamSync returns upstream sync items (nil on error) and headers.
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

	// Devices send a plural ReadingStates array, whole-book ProgressPercent 0-100,
	// and Location as a {Source,Type,Value} object.
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

	// No prior state is fine: buildKoboState treats nil as 0%.
	state, err := app.Services.Books.GetReadingState(r.Context(), userID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	koboWriteJSON(w, buildKoboState(bookID.String(), state))
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

// parseKoboLocation reads CurrentBookmark.Location as {Source,Type,Value},
// falling back to a bare string.
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

// koboStatusForPercent derives StatusInfo.Status from percent alone.
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

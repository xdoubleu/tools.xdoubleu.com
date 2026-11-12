//nolint:testpackage // fakes the unexported-field Deps seam from inside the package
package kobo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/logging"
)

// fakeLibrary implements Library over in-memory data; unused methods panic
// through the nil embedded interface.
type fakeLibrary struct {
	Library
	books     []models.KoboSyncBook
	removals  []models.KoboRemoval
	states    map[uuid.UUID]*models.BookReadingState
	userBooks map[uuid.UUID]bool

	mu       sync.Mutex
	synced   []uuid.UUID
	progress []models.BookReadingState
	linked   []models.KoboStoreBook
	recorded []models.KoboStoreReading
	outcome  models.KoboStoreOutcome
}

func (f *fakeLibrary) ListKoboSyncBooks(
	context.Context, string,
) ([]models.KoboSyncBook, error) {
	return f.books, nil
}

func (f *fakeLibrary) GetKoboSyncBook(
	_ context.Context, _ string, id uuid.UUID,
) (models.KoboSyncBook, error) {
	for _, b := range f.books {
		if b.BookID == id {
			return b, nil
		}
	}
	return models.KoboSyncBook{}, database.ErrResourceNotFound
}

func (f *fakeLibrary) ListKoboRemovals(
	context.Context, string,
) ([]models.KoboRemoval, error) {
	return f.removals, nil
}

func (f *fakeLibrary) UpdateKoboLastSynced(
	_ context.Context, _ string, id uuid.UUID, _ int16, _ string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.synced = append(f.synced, id)
	return nil
}

func (f *fakeLibrary) WarmUserCovers(context.Context, string, []uuid.UUID) {}

func (f *fakeLibrary) GetUserBook(
	_ context.Context, _ string, id uuid.UUID,
) (*models.UserBook, error) {
	if f.userBooks[id] {
		return &models.UserBook{}, nil //nolint:exhaustruct // identity only
	}
	return nil, database.ErrResourceNotFound
}

func (f *fakeLibrary) ListReadingStates(
	context.Context, string,
) (map[uuid.UUID]*models.BookReadingState, error) {
	return f.states, nil
}

func (f *fakeLibrary) GetReadingState(
	_ context.Context, _ string, id uuid.UUID,
) (*models.BookReadingState, error) {
	if st, ok := f.states[id]; ok {
		return st, nil
	}
	return nil, database.ErrResourceNotFound
}

func (f *fakeLibrary) ListKoboDeviceHeld(
	context.Context, string,
) (map[uuid.UUID]time.Time, error) {
	return map[uuid.UUID]time.Time{}, nil
}

func (f *fakeLibrary) SyncKoboDeviceReadingStates(
	_ context.Context,
	_ string,
	ids []uuid.UUID,
	states map[uuid.UUID]*models.BookReadingState,
	_ map[uuid.UUID]time.Time,
	_ map[uuid.UUID]bool,
) ([]*models.BookReadingState, error) {
	var out []*models.BookReadingState
	for _, id := range ids {
		if st := states[id]; st != nil {
			out = append(out, st)
		}
	}
	return out, nil
}

func (f *fakeLibrary) UpdateKoboReadingProgress(
	_ context.Context, _ string, st models.BookReadingState,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progress = append(f.progress, st)
	return nil
}

func (f *fakeLibrary) ClearKoboLocation(
	context.Context, string, uuid.UUID, models.KoboLocation,
) error {
	return nil
}

func (f *fakeLibrary) LinkKoboStoreBooks(
	_ context.Context, _ string, sb []models.KoboStoreBook,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.linked = append(f.linked, sb...)
	return nil
}

func (f *fakeLibrary) RecordKoboStoreReading(
	_ context.Context, _ string, rd models.KoboStoreReading,
) (models.KoboStoreOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recorded = append(f.recorded, rd)
	if f.outcome == models.KoboStoreUnrecorded && len(f.linked) > 0 {
		return models.KoboStoreMirrored, nil
	}
	return f.outcome, nil
}

type fakePositions struct{ Positions }

func (fakePositions) KoboLocations(
	context.Context,
	string,
	map[uuid.UUID]*models.BookReadingState,
	map[uuid.UUID]bool,
) (map[uuid.UUID]*models.KoboLocation, map[uuid.UUID]bool) {
	return nil, nil
}

func (fakePositions) KoboPosition(
	context.Context, string, uuid.UUID, models.KoboLocation, bool,
) (*models.ReadingPosition, bool) {
	return nil, true
}

func (fakePositions) BackfillKoboPosition(
	context.Context,
	string,
	uuid.UUID,
	models.KoboLocation,
	func(context.Context, models.ReadingPosition) error,
) {
}

type fakeKEPUBs struct {
	stale     bool
	requested []uuid.UUID
}

func (f *fakeKEPUBs) IsKEPUBStale(int16) bool { return f.stale }

func (f *fakeKEPUBs) Request(_ context.Context, _ string, id uuid.UUID) error {
	f.requested = append(f.requested, id)
	return nil
}

// inlineBackground runs work synchronously so tests can assert on it.
type inlineBackground struct{}

func (inlineBackground) Go(ctx context.Context, _ string, fn func(context.Context)) {
	fn(ctx)
}

func newLib(opts ...func(*fakeLibrary)) *fakeLibrary {
	//nolint:exhaustruct // zero values are the intended initial state
	f := &fakeLibrary{}
	for _, o := range opts {
		o(f)
	}
	return f
}

func newKEPUBs(stale bool) *fakeKEPUBs {
	//nolint:exhaustruct // nothing requested yet
	return &fakeKEPUBs{stale: stale}
}

func newProtocol(lib *fakeLibrary, kep *fakeKEPUBs, storeURL string) *Protocol {
	return NewProtocol(Deps{
		Library:          lib,
		Positions:        fakePositions{Positions: nil},
		KEPUBs:           kep,
		Background:       inlineBackground{},
		Logger:           logging.NewNopLogger(),
		StoreBaseURL:     storeURL,
		PublicAPIBaseURL: "",
	})
}

func syncBook(id uuid.UUID) models.KoboSyncBook {
	//nolint:exhaustruct // only the fields the protocol reads
	return models.KoboSyncBook{
		BookID:            id,
		Title:             "T",
		Size:              42,
		ConverterVersion:  3,
		KoboSyncEnabledAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func decodeEntries(t *testing.T, raw []json.RawMessage) []map[string]json.RawMessage {
	t.Helper()
	out := make([]map[string]json.RawMessage, len(raw))
	for i, r := range raw {
		require.NoError(t, json.Unmarshal(r, &out[i]))
	}
	return out
}

func TestSync_MergesUpstreamBooksStatesAndRemovals(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/library/sync", r.URL.Path)
			assert.Equal(t, "a=b", r.URL.RawQuery)
			assert.Equal(t, "Bearer x", r.Header.Get("Authorization"))
			w.Header().Set(HeaderSync, "continue")
			_, _ = w.Write([]byte(`[{"NewEntitlement":{
				"BookEntitlement":{"Id":"store1","Accessibility":"Full"},
				"BookMetadata":{"Title":"S"}}}]`))
		}))
	t.Cleanup(upstream.Close)

	bookID, removedID := uuid.New(), uuid.New()
	lib := newLib(func(f *fakeLibrary) {
		f.books = []models.KoboSyncBook{syncBook(bookID)}
		f.removals = []models.KoboRemoval{{BookID: removedID, RemovedAt: time.Now()}}
		f.states = map[uuid.UUID]*models.BookReadingState{
			bookID: {BookID: bookID, Percent: 40, UpdatedAt: time.Now()},
		}
	})
	kep := newKEPUBs(true)
	p := newProtocol(lib, kep, upstream.URL)

	hdr := http.Header{"Authorization": {"Bearer x"}}
	res, err := p.Sync(t.Context(), SyncRequest{
		UserID: "u", DeviceID: "d", LibraryBase: "https://h/lib",
		Header: hdr, RawQuery: "a=b",
	})
	require.NoError(t, err)
	assert.True(t, res.UpstreamFetched)
	assert.Empty(t, res.Upstream.Note)
	assert.Equal(t, "continue", res.Upstream.PagingHeaders().Get(HeaderSync))

	// upstream item, our book, its changed state, the removal
	require.Len(t, res.Entries, 4)
	entries := decodeEntries(t, res.Entries)
	assert.Contains(t, entries[0], "NewEntitlement")
	assert.Contains(t, entries[1], "NewEntitlement")
	assert.Contains(t, entries[2], "ChangedReadingState")
	assert.Contains(t, entries[3], "ChangedEntitlement")
	assert.Contains(t, string(res.Entries[1]), "https://h/lib/"+bookID.String()+"/file")
	assert.Contains(t, string(res.Entries[3]), `"IsRemoved":true`)

	assert.Equal(t, []uuid.UUID{bookID}, kep.requested)
	assert.Equal(t, []uuid.UUID{bookID}, lib.synced)
	require.Len(t, lib.linked, 1)
	assert.Equal(t, "store1", lib.linked[0].EntitlementID)
}

func TestSync_UpstreamFailureKeepsOurBooksAndDropsPaging(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(HeaderSync, "continue")
			w.WriteHeader(http.StatusUnauthorized)
		}))
	t.Cleanup(upstream.Close)

	bookID := uuid.New()
	lib := newLib(func(f *fakeLibrary) {
		f.books = []models.KoboSyncBook{syncBook(bookID)}
		f.states = map[uuid.UUID]*models.BookReadingState{}
	})
	p := newProtocol(lib, newKEPUBs(false), upstream.URL)

	res, err := p.Sync(t.Context(), SyncRequest{
		UserID: "u", DeviceID: "d", LibraryBase: "", Header: http.Header{}, RawQuery: "",
	})
	require.NoError(t, err)
	assert.Equal(t, "upstream 401 (no Authorization forwarded)", res.Upstream.Note)
	assert.Empty(t, res.Upstream.PagingHeaders())
	assert.Len(t, res.Entries, 1)
	assert.Empty(t, lib.linked)
}

func TestSync_EmptyLibraryEncodesEmptyArray(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) }))
	t.Cleanup(upstream.Close)

	p := newProtocol(newLib(), newKEPUBs(false), upstream.URL)
	res, err := p.Sync(t.Context(), SyncRequest{
		UserID: "u", DeviceID: "d", LibraryBase: "", Header: http.Header{}, RawQuery: "",
	})
	require.NoError(t, err)
	out, _ := json.Marshal(res.Entries)
	assert.JSONEq(t, `[]`, string(out), "nil would encode as null and hang the device")
}

func TestSync_RegeneratedKEPUBGoesOutAsChangedEntitlement(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) }))
	t.Cleanup(upstream.Close)

	b := syncBook(uuid.New())
	old := int16(2)
	b.LastSyncedConverterVersion = &old
	p := newProtocol(
		newLib(func(f *fakeLibrary) { f.books = []models.KoboSyncBook{b} }),
		newKEPUBs(false), upstream.URL,
	)

	res, err := p.Sync(t.Context(), SyncRequest{
		UserID: "u", DeviceID: "d", LibraryBase: "", Header: http.Header{}, RawQuery: "",
	})
	require.NoError(t, err)
	require.Len(t, res.Entries, 1)
	assert.Contains(t, string(res.Entries[0]), "ChangedEntitlement")
}

func TestBaseURLs(t *testing.T) {
	p := newProtocol(newLib(), newKEPUBs(false), "")
	assert.Equal(t, "https://fwd/books/kobo/tok",
		p.CoverBase("/books/kobo/tok/v1/initialization", "fwd", "h"))
	assert.Equal(t, "https://h/books/kobo/tok/v1/library",
		p.LibraryBase("/books/kobo/tok/v1/library/sync", "", "h"))

	p.d.PublicAPIBaseURL = "https://public/api/"
	assert.Equal(t, "https://public/api/books/kobo/tok/v1/library",
		p.LibraryBase("/books/kobo/tok/v1/library/x/metadata", "fwd", "h"))
}

func TestInit_PointsOwnedKeysAtUs(t *testing.T) {
	p := newProtocol(newLib(), newKEPUBs(false), "")
	resp := p.Init("https://h/kobo/tok")

	assert.Equal(t, "https://h/kobo/tok/v1/library/sync", resp.Resources["library_sync"])
	assert.Equal(t, "https://h/kobo/tok/v1/library/{Ids}/state",
		resp.Resources["reading_state"])
	assert.Equal(t, "https://h/kobo/tok", resp.Resources["image_host"])
	assert.Greater(t, len(resp.Resources), 20, "full catalog, not a truncated set")
	assert.Equal(t, "Kobo", resp.Settings.UserAgent)
}

func TestMetadataAndHasSyncBook(t *testing.T) {
	id := uuid.New()
	p := newProtocol(
		newLib(func(f *fakeLibrary) {
			f.books = []models.KoboSyncBook{syncBook(id)}
		}),
		newKEPUBs(false), "",
	)

	meta, err := p.Metadata(t.Context(), "u", id, "https://h/lib")
	require.NoError(t, err)
	require.Len(t, meta, 1)
	assert.Equal(t, CoverImageID(id), meta[0].CoverImageID)

	_, err = p.Metadata(t.Context(), "u", uuid.New(), "https://h/lib")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)

	ok, err := p.HasSyncBook(t.Context(), "u", id)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = p.HasSyncBook(t.Context(), "u", uuid.New())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestLibraryBookID(t *testing.T) {
	id := uuid.New()
	p := newProtocol(
		newLib(func(f *fakeLibrary) { f.userBooks = map[uuid.UUID]bool{id: true} }),
		newKEPUBs(false), "",
	)

	got, ok, err := p.LibraryBookID(t.Context(), "u", id.String())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, id, got)

	_, ok, err = p.LibraryBookID(t.Context(), "u", uuid.NewString())
	require.NoError(t, err)
	assert.False(t, ok, "unknown book is a store book")

	_, ok, err = p.LibraryBookID(t.Context(), "u", "store-entitlement")
	require.NoError(t, err)
	assert.False(t, ok, "non-UUID id is a store book")
}

func TestGetState(t *testing.T) {
	id := uuid.New()
	updated := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	lib := newLib(func(f *fakeLibrary) {
		f.states = map[uuid.UUID]*models.BookReadingState{
			id: {BookID: id, Percent: 100, UpdatedAt: updated},
		}
	})
	p := newProtocol(lib, newKEPUBs(false), "")

	st, err := p.GetState(t.Context(), "u", id)
	require.NoError(t, err)
	assert.Equal(t, 100, st.CurrentBookmark.ProgressPercent)
	assert.Equal(t, "Finished", st.StatusInfo.Status)
	assert.Equal(t, "2026-05-06T07:08:09Z", st.LastModified)

	none, err := p.GetState(t.Context(), "u", uuid.New())
	require.NoError(t, err)
	assert.Equal(t, epoch, none.LastModified)
	assert.Equal(t, "ReadyToRead", none.StatusInfo.Status)
}

func TestPutState(t *testing.T) {
	id := uuid.New()
	lib := newLib()
	p := newProtocol(lib, newKEPUBs(false), "")

	body := `{"ReadingStates":[{"CurrentBookmark":{
		"LastModified":"2026-01-01T00:00:00Z","ProgressPercent":55,
		"Location":{"Source":"c1","Type":"KoboSpan","Value":"kobo.1.1"}},
		"StatusInfo":{"Status":"Reading"}}]}`
	ack, err := p.PutState(t.Context(), "u", "d", id, strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, "Success", ack.RequestResult)
	require.Len(t, ack.UpdateResults, 1)
	assert.Equal(t, id.String(), ack.UpdateResults[0].EntitlementId)
	assert.Equal(t, "Ignored", ack.UpdateResults[0].StatisticsResult.Result)

	require.Len(t, lib.progress, 1)
	assert.Equal(t, 55, lib.progress[0].Percent)
	require.NotNil(t, lib.progress[0].KoboLocation)
	assert.Equal(t, "kobo.1.1", lib.progress[0].KoboLocation.Value)

	empty, err := p.PutState(t.Context(), "u", "d", id, strings.NewReader(`{}`))
	require.NoError(t, err)
	assert.Empty(t, empty.UpdateResults)

	_, err = p.PutState(t.Context(), "u", "d", id, strings.NewReader(`nope`))
	assert.ErrorIs(t, err, ErrBadStateBody)
}

func TestPutState_FinishedKeepsValueOnly(t *testing.T) {
	lib := newLib()
	p := newProtocol(lib, newKEPUBs(false), "")

	body := `{"ReadingStates":[{"CurrentBookmark":{"ProgressPercent":100,
		"Location":{"Source":"first","Type":"KoboSpan","Value":"v"}},
		"StatusInfo":{"Status":"Finished"}}]}`
	_, err := p.PutState(t.Context(), "u", "d", uuid.New(), strings.NewReader(body))
	require.NoError(t, err)
	require.Len(t, lib.progress, 1)
	assert.Nil(t, lib.progress[0].KoboLocation)
	require.NotNil(t, lib.progress[0].Location)
	assert.Equal(t, "v", *lib.progress[0].Location)
}

func TestMirrorStoreStateAsync_LearnsUnknownBookFromStoreMetadata(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/library/store1/metadata", r.URL.Path)
			assert.Equal(t, "Bearer x", r.Header.Get("Authorization"))
			assert.Empty(t, r.Header.Get("Content-Type"))
			_, _ = w.Write([]byte(`[{"Title":"S","Isbn":"9780000000001"}]`))
		}))
	t.Cleanup(upstream.Close)

	lib := newLib(func(f *fakeLibrary) { f.outcome = models.KoboStoreUnrecorded })
	p := newProtocol(lib, newKEPUBs(false), upstream.URL)

	hdr := http.Header{"Authorization": {"Bearer x"}, "Content-Type": {"json"}}
	body := []byte(`{"ReadingStates":[{"CurrentBookmark":{"ProgressPercent":50},
		"StatusInfo":{"Status":"Reading"}}]}`)
	p.MirrorStoreStateAsync(t.Context(), "u", "store1", hdr, body)

	require.Len(t, lib.linked, 1)
	assert.Equal(t, "S", lib.linked[0].Title)
	assert.Len(t, lib.recorded, 2, "recorded before and after learning the book")
	assert.Equal(t, 50, lib.recorded[1].Percent)
}

func TestMirrorStoreState_IgnoresUnparsableBody(t *testing.T) {
	lib := newLib()
	p := newProtocol(lib, newKEPUBs(false), "")
	p.mirrorStoreState(t.Context(), "u", "x", http.Header{}, []byte(`nope`))
	p.mirrorStoreState(t.Context(), "u", "x", http.Header{}, []byte(`{}`))
	assert.Empty(t, lib.recorded)
}

func TestFetchStoreMetadataFailures(t *testing.T) {
	status := http.StatusInternalServerError
	body := `[]`
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		}))
	t.Cleanup(upstream.Close)
	p := newProtocol(newLib(), newKEPUBs(false), upstream.URL)

	_, err := p.fetchStoreMetadata(t.Context(), http.Header{}, "id")
	require.ErrorContains(t, err, "upstream 500 (no Authorization forwarded)")

	status = http.StatusOK
	_, err = p.fetchStoreMetadata(t.Context(), http.Header{}, "id")
	require.ErrorContains(t, err, "empty metadata")

	body = `nope`
	_, err = p.fetchStoreMetadata(t.Context(), http.Header{}, "id")
	require.Error(t, err)
}

func TestProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Equal(t, "/v1/x?q=1", r.URL.RequestURI())
			assert.Equal(t, "payload", string(b))
			assert.Equal(t, "Bearer x", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusTeapot)
		}))
	t.Cleanup(upstream.Close)
	p := newProtocol(newLib(), newKEPUBs(false), upstream.URL)

	resp, err := p.Proxy(
		t.Context(), http.MethodPut, "/v1/x", "q=1",
		http.Header{"Authorization": {"Bearer x"}, "Accept-Encoding": {"br"}},
		strings.NewReader("payload"),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTeapot, resp.StatusCode)

	_, err = p.Proxy(t.Context(), "BAD METHOD", "/", "", http.Header{}, nil)
	assert.ErrorIs(t, err, ErrBuildRequest)
}

func TestCoverIDsAndUpstreamURL(t *testing.T) {
	id := uuid.New()
	got, err := CoverBookID(CoverImageID(id))
	require.NoError(t, err)
	assert.Equal(t, id, got)
	got, err = CoverBookID(id.String())
	require.NoError(t, err)
	assert.Equal(t, id, got, "bare id from before the revision suffix")
	_, err = CoverBookID("store-image")
	require.Error(t, err)

	plain := UpstreamCoverURL(CoverParams{
		ImageID: "img", Width: "100", Height: "200", Quality: "", Greyscale: "",
	})
	assert.Contains(t, plain, "img")
	assert.NotContains(t, plain, "{")
	quality := UpstreamCoverURL(CoverParams{
		ImageID: "img", Width: "100", Height: "200", Quality: "90", Greyscale: "true",
	})
	assert.Contains(t, quality, "90")
	assert.NotEqual(t, plain, quality)
}

func TestDeviceLacksAndHashToken(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	held := map[uuid.UUID]time.Time{id: now}
	//nolint:exhaustruct // only UpdatedAt matters
	newer := &models.BookReadingState{UpdatedAt: now.Add(time.Second)}
	//nolint:exhaustruct // only UpdatedAt matters
	same := &models.BookReadingState{UpdatedAt: now}

	assert.True(t, DeviceLacks(map[uuid.UUID]time.Time{}, id, nil))
	assert.True(t, DeviceLacks(held, id, newer))
	assert.False(t, DeviceLacks(held, id, same))
	assert.False(t, DeviceLacks(held, id, nil))

	assert.Equal(t,
		"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		HashToken("hello"))
}

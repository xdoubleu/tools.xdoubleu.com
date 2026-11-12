package kobo

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/threading"
)

// SyncRequest is one device's /v1/library/sync call.
type SyncRequest struct {
	UserID, DeviceID string
	// LibraryBase prefixes the download URLs in book metadata.
	LibraryBase string
	// Header and RawQuery are the device's, forwarded to the upstream store.
	Header   http.Header
	RawQuery string
}

// SyncResult is the merged sync page. Upstream is only meaningful once
// UpstreamFetched; the caller records its headers and note for the debug log
// even when Sync then fails.
type SyncResult struct {
	Entries         []json.RawMessage
	Upstream        Upstream
	UpstreamFetched bool
}

// Sync merges our kobo-sync books, reading-state changes and removals into
// the upstream store's entitlements.
func (p *Protocol) Sync(ctx context.Context, req SyncRequest) (SyncResult, error) {
	var res SyncResult
	books, err := p.d.Library.ListKoboSyncBooks(ctx, req.UserID)
	if err != nil {
		return res, err
	}

	// Warm covers in the background (detached so a slow source never stalls
	// the sync) so the device's subsequent cover fetches during its update
	// phase are fast R2 hits instead of blocking live fetches.
	p.d.Background.Go(ctx, "kobo-cover-warm", func(ctx context.Context) {
		p.warmCovers(ctx, req.UserID, books)
	})

	stateByBook, err := p.d.Library.ListReadingStates(ctx, req.UserID)
	if err != nil {
		return res, err
	}

	// The upstream fetch overlaps the span translation's wait. Not on
	// Background: its slots must never hold up the response. The note stands
	// if the fetch panics.
	upstream := Upstream{Items: nil, Hdrs: nil, Note: "upstream: fetch panicked"}
	upstreamDone := make(chan struct{})
	go func() {
		defer close(upstreamDone)
		defer threading.Recover(ctx, p.d.Logger, "kobo-upstream-sync")
		upstream = p.fetchUpstreamSync(ctx, req.RawQuery, req.Header)
	}()

	held, pending, err := p.syncSpans(
		ctx, req.UserID, req.DeviceID, books, stateByBook,
	)
	if err != nil {
		return res, err
	}

	ourEntries := make([]json.RawMessage, len(books))
	for i, b := range books {
		ourEntries[i] = p.buildSyncEntry(
			ctx, req.UserID, b, stateByBook, req.LibraryBase,
		)
	}
	removals, err := p.d.Library.ListKoboRemovals(ctx, req.UserID)
	if err != nil {
		return res, err
	}
	removalEntries := make([]json.RawMessage, len(removals))
	for i, rm := range removals {
		removalEntries[i] = buildRemovalEntry(rm)
	}

	<-upstreamDone
	res.Upstream, res.UpstreamFetched = upstream, true
	if upstream.Note != "" {
		p.d.Logger.Warn("kobo upstream store sync failed",
			"device_id", req.DeviceID, "note", upstream.Note)
	}

	// Last before responding: this marks the states as delivered.
	stateEntries, err := p.changedReadingStates(
		ctx, req.DeviceID, books, stateByBook, held, pending,
	)
	if err != nil {
		return res, err
	}

	p.linkStoreBooks(ctx, req.UserID, upstream)

	all := append(upstream.Items, ourEntries...) //nolint:gocritic // intentional
	all = append(all, stateEntries...)
	all = append(all, removalEntries...)
	if all == nil {
		// A nil slice encodes as null, which hangs the firmware at "Checking for
		// updates…".
		all = []json.RawMessage{}
	}
	res.Entries = all
	return res, nil
}

// linkStoreBooks links a merged upstream page's store books to the
// library in the background.
func (p *Protocol) linkStoreBooks(
	ctx context.Context,
	userID string,
	upstream Upstream,
) {
	if upstream.Note != "" || len(upstream.Items) == 0 {
		return
	}
	p.d.Background.Go(ctx, "kobo-store-sync", func(ctx context.Context) {
		p.recordStoreSync(ctx, userID, upstream.Items)
	})
}

// warmCovers caches every book's cover so the device's per-cover fetches
// during its update phase are fast R2 hits.
func (p *Protocol) warmCovers(
	ctx context.Context,
	userID string,
	books []models.KoboSyncBook,
) {
	ids := make([]uuid.UUID, len(books))
	for i, b := range books {
		ids[i] = b.BookID
	}
	p.d.Library.WarmUserCovers(ctx, userID, ids)
}

// buildSyncEntry builds one book's entitlement. RevisionId stays the bare
// book UUID: a varying one adds a second copy on the device. A regenerated
// KEPUB or an undelivered CoverImageId goes out as ChangedEntitlement, since
// the device ignores metadata in a NewEntitlement it already holds.
func (p *Protocol) buildSyncEntry(
	ctx context.Context,
	userID string,
	b models.KoboSyncBook,
	stateByBook map[uuid.UUID]*models.BookReadingState,
	libraryBase string,
) json.RawMessage {
	id := b.BookID.String()
	// KoboSyncEnabledAt keeps the payload byte-identical across syncs; time.Now()
	// makes the firmware recreate the entitlement each time (books flicker).
	enabled := b.KoboSyncEnabledAt.UTC().Format(time.RFC3339)
	coverID := CoverImageID(b.BookID)
	isChanged := isReplace(b) || coverChanged(b, coverID)

	if p.d.KEPUBs.IsKEPUBStale(b.ConverterVersion) {
		// Queued for the workers role: the sync request must never wait on a
		// PDF conversion (ADR-0017).
		if err := p.d.KEPUBs.Request(ctx, userID, b.BookID); err != nil {
			p.d.Logger.ErrorContext(ctx, "queue kepub regeneration",
				"book_id", b.BookID, "err", err)
		}
	}

	entry := syncEntry{
		BookEntitlement: bookEntitlement{
			Accessibility:   accessFull,
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
		BookMetadata: buildMetadata(b, libraryBase),
		// ReadingState must be non-nil so the firmware PUTs state on progress.
		ReadingState: buildState(id, stateByBook[b.BookID]),
	}

	var raw []byte
	if isChanged {
		raw, _ = json.Marshal(changedEntitlement{ChangedEntitlement: entry})
	} else {
		raw, _ = json.Marshal(newEntitlement{NewEntitlement: entry})
	}

	if b.LastSyncedConverterVersion == nil ||
		*b.LastSyncedConverterVersion != b.ConverterVersion ||
		b.LastSyncedCoverImageID == nil || *b.LastSyncedCoverImageID != coverID {
		if updErr := p.d.Library.UpdateKoboLastSynced(
			ctx, userID, b.BookID, b.ConverterVersion, coverID,
		); updErr != nil {
			p.d.Logger.Error("failed to update kobo last synced state",
				"error", updErr, "bookID", b.BookID)
		}
	}

	return raw
}

// buildRemovalEntry builds a ChangedEntitlement telling the device to
// delete a synced book; the catalog row may be gone, so metadata is minimal.
// Unverified on a real device.
func buildRemovalEntry(rm models.KoboRemoval) json.RawMessage {
	id := rm.BookID.String()
	removed := rm.RemovedAt.UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(changedEntitlement{
		ChangedEntitlement: syncEntry{
			BookEntitlement: bookEntitlement{
				Accessibility:   accessFull,
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
			BookMetadata: BookMetadata{ //nolint:exhaustruct // removal: no
				// download/content details to give — the book is gone.
				RevisionId: id,
			},
			ReadingState: nil,
		},
	})
	return raw
}

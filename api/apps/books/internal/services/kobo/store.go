package kobo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// recordStoreSync links the store books of an upstream sync page to the
// library and mirrors their reading states. Runs detached from the request.
func (p *Protocol) recordStoreSync(
	ctx context.Context,
	userID string,
	items []json.RawMessage,
) {
	storeBooks, readings := parseStoreItems(items)
	if err := p.d.Library.LinkKoboStoreBooks(ctx, userID, storeBooks); err != nil {
		p.d.Logger.WarnContext(ctx, "linking kobo store books failed", "err", err)
		return
	}
	for _, rd := range readings {
		if _, err := p.d.Library.RecordKoboStoreReading(ctx, userID, rd); err != nil {
			p.d.Logger.WarnContext(ctx, "mirroring kobo store reading state failed",
				"entitlement_id", rd.EntitlementID, "err", err)
		}
	}
}

// MirrorStoreStateAsync records a store book's PUT state on its library match
// in the background; hdr and body are the device's request, already proxied.
func (p *Protocol) MirrorStoreStateAsync(
	ctx context.Context,
	userID, id string,
	hdr http.Header,
	body []byte,
) {
	p.d.Background.Go(ctx, "kobo-store-state", func(ctx context.Context) {
		p.mirrorStoreState(ctx, userID, id, hdr, body)
	})
}

// mirrorStoreState records a store book's PUT state on its library match,
// first learning the book from the store's metadata if it's new to us.
func (p *Protocol) mirrorStoreState(
	ctx context.Context,
	userID, id string,
	hdr http.Header,
	body []byte,
) {
	var put struct {
		ReadingStates []storeState `json:"ReadingStates"`
	}
	if json.Unmarshal(body, &put) != nil || len(put.ReadingStates) == 0 {
		return
	}
	state := put.ReadingStates[len(put.ReadingStates)-1]
	reading := state.toReading(id)
	reading.EntitlementID = id

	outcome, err := p.d.Library.RecordKoboStoreReading(ctx, userID, reading)
	if err == nil && outcome == models.KoboStoreUnrecorded {
		var sb *models.KoboStoreBook
		if sb, err = p.fetchStoreMetadata(ctx, hdr, id); err == nil {
			err = p.d.Library.LinkKoboStoreBooks(
				ctx, userID, []models.KoboStoreBook{*sb},
			)
		}
		if err == nil {
			outcome, err = p.d.Library.RecordKoboStoreReading(ctx, userID, reading)
		}
	}
	switch {
	case err != nil:
		p.d.Logger.WarnContext(ctx, "mirroring kobo store reading state failed",
			"entitlement_id", id, "err", err)
	case outcome != models.KoboStoreMirrored:
		p.d.Logger.InfoContext(ctx, "kobo store reading state not mirrored",
			"entitlement_id", id, "percent", reading.Percent, "outcome", outcome,
			"progress_percent", state.CurrentBookmark.ProgressPercent,
			"status", state.StatusInfo.Status)
	}
}

// fetchStoreMetadata reads a store book's metadata from the Kobo store.
// Ownership is unknown from metadata alone.
func (p *Protocol) fetchStoreMetadata(
	ctx context.Context,
	src http.Header,
	id string,
) (*models.KoboStoreBook, error) {
	targetURL := p.d.StoreBaseURL + "/v1/library/" + url.PathEscape(id) + "/metadata"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header = src.Clone()
	req.Header.Del("Accept-Encoding")
	req.Header.Del("Content-Length")
	req.Header.Del("Content-Type")
	forwardAuth(req.Header, src)

	resp, err := upstreamClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(UpstreamNote(resp.StatusCode, src))
	}

	var metas []storeMetadata
	if err = json.NewDecoder(resp.Body).Decode(&metas); err != nil {
		return nil, err
	}
	if len(metas) == 0 {
		return nil, errors.New("upstream: empty metadata")
	}
	sb := metas[0].toStoreBook(id, nil)
	return &sb, nil
}

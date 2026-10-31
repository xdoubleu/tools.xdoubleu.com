package kobo

import (
	_ "embed"
	"encoding/json"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// Kobo sync paging headers.
const (
	HeaderSync     = "x-kobo-sync"
	HeaderSyncMode = "x-kobo-sync-mode"
	//nolint:gosec // a header name, not a credential
	HeaderSyncToken = "x-kobo-synctoken"
)

// accessFull is the Accessibility of an entitlement readable in full.
const accessFull = "Full"

// epoch is LastModified when no server state exists. time.Now() would make
// the firmware overwrite local progress with 0% and never PUT state.
const epoch = "1970-01-01T00:00:00Z"

// statusFinished is the StatusInfo.Status of a finished book, whose
// bookmark Source is the first resource rather than the reading position.
const statusFinished = "Finished"

// statusReading is the StatusInfo.Status of an opened, unfinished book.
const statusReading = "Reading"

const resultSuccess = "Success"

// InitResponse is the /v1/initialization payload.
type InitResponse struct {
	Resources map[string]any `json:"Resources"`
	Settings  settings       `json:"Settings"`
	TokenList []string       `json:"TokenList"`
}

type settings struct {
	SynchronizationDelay int    `json:"SynchronizationDelay"`
	TestEmailAddress     string `json:"TestEmailAddress"`
	UserAgent            string `json:"UserAgent"`
}

//go:embed resources.json
var catalogJSON []byte

// catalog is the full Kobo onestore resource catalog, decoded once from
// resources.json. The payload mirrors Calibre-Web's NATIVE_KOBO_RESOURCES:
// the firmware reads every key from /v1/initialization, and a truncated set
// (like the six keys #2298 served) returns 200 on init but never advances the
// device to /v1/library/sync. Keys we own are overridden in Init to point at
// our endpoints; the rest keep the upstream Kobo store URLs (proxied).
func catalog() map[string]any {
	var c map[string]any
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic("kobo resources.json: " + err.Error())
	}
	return c
}

type syncEntry struct {
	BookEntitlement bookEntitlement `json:"BookEntitlement"`
	BookMetadata    BookMetadata    `json:"BookMetadata"`
	ReadingState    *ReadingState   `json:"ReadingState"`
}

// newEntitlement wraps an entry in the change-type key the firmware
// requires; a bare payload is silently ignored.
type newEntitlement struct {
	NewEntitlement syncEntry `json:"NewEntitlement"`
}

// changedEntitlement signals a removal of a previously synced book.
type changedEntitlement struct {
	ChangedEntitlement syncEntry `json:"ChangedEntitlement"`
}

type bookEntitlement struct {
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

// BookMetadata is the Kobo BookMetadata payload.
type BookMetadata struct {
	Title       string `json:"Title"`
	ContentType string `json:"ContentType"`
	//nolint:revive // Kobo protocol field name
	RevisionId   string        `json:"RevisionId"`
	Language     string        `json:"Language"`
	DownloadUrls []downloadURL `json:"DownloadUrls"`
	// CoverImageId names the image the device requests from
	// image_url(_quality)_template; see CoverImageID.
	CoverImageID string `json:"CoverImageId"`
}

type downloadURL struct {
	Format   string `json:"Format"`
	Size     int64  `json:"Size"`
	URL      string `json:"Url"`
	Platform string `json:"Platform"`
}

// ReadingState is the Kobo ReadingState payload.
type ReadingState struct {
	CurrentBookmark bookmark `json:"CurrentBookmark"`
	//nolint:revive // Kobo protocol field name
	EntitlementId     string     `json:"EntitlementId"`
	LastModified      string     `json:"LastModified"`
	PriorityTimestamp string     `json:"PriorityTimestamp"`
	StatusInfo        statusInfo `json:"StatusInfo"`
}

type bookmark struct {
	LastModified    string `json:"LastModified"`
	ProgressPercent int    `json:"ProgressPercent"`
	// ContentSourceProgressPercent is within-chapter on devices; we mirror the
	// whole-book percent.
	ContentSourceProgressPercent int                  `json:"ContentSourceProgressPercent"`
	Location                     *models.KoboLocation `json:"Location,omitempty"`
}

type statusInfo struct {
	LastModified string `json:"LastModified"`
	Status       string `json:"Status"`
	//nolint:revive // Kobo protocol field name
	TimestampId string `json:"TimestampId"`
}

// changedReadingState pushes a new position for a book already on the
// device; it ignores ReadingState in a repeated NewEntitlement.
type changedReadingState struct {
	ChangedReadingState struct {
		ReadingState *ReadingState `json:"ReadingState"`
	} `json:"ChangedReadingState"`
}

// StateAck answers a state PUT. Without a per-section Success the device
// keeps its local copy authoritative and ignores server positions.
type StateAck struct {
	RequestResult string         `json:"RequestResult"`
	UpdateResults []updateResult `json:"UpdateResults"`
}

type updateResult struct {
	//nolint:revive // Kobo protocol field name
	EntitlementId         string `json:"EntitlementId"`
	CurrentBookmarkResult result `json:"CurrentBookmarkResult"`
	StatisticsResult      result `json:"StatisticsResult"`
	StatusInfoResult      result `json:"StatusInfoResult"`
}

type result struct {
	Result string `json:"Result"`
}

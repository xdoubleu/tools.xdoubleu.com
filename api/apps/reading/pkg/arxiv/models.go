package arxiv

import "time"

// Paper is the metadata for one arXiv paper.
type Paper struct {
	// ID is the canonical versionless arXiv id, e.g. "2401.12345" or
	// "math.GT/0309136".
	ID        string
	Title     string
	Authors   []string
	Abstract  string
	Published time.Time
	// PDFURL is the canonical PDF download URL.
	PDFURL string
	// AbsURL is the canonical abstract page URL — used as the item's
	// source_url so /abs/, /pdf/, and DOI pastes dedup to one row.
	AbsURL string
}

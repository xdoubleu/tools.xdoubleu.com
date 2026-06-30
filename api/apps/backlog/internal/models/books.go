package models

import (
	"math"
	"time"

	"github.com/google/uuid"
)

const (
	StatusToRead  = "to-read"
	StatusReading = "currently-reading"
	StatusRead    = "read"
	StatusDropped = "dropped"
)

const (
	TagOwnPhysical   = "own-physical"
	TagOwnDigital    = "own-digital"
	TagFavourite     = "favourite"
	TagKoboSync      = "kobo-sync"
	TagKoboFormatPDF = "kobo-format-pdf"
)

const (
	ProgressModePages   = "pages"
	ProgressModePercent = "percent"
)

// MaxProgressPercent is the upper bound for a reading-progress percentage.
const MaxProgressPercent = 100

// IsSpecialTag reports whether a tag has reserved UI treatment (ownership, favourite).
func IsSpecialTag(t string) bool {
	return t == TagOwnPhysical || t == TagOwnDigital || t == TagFavourite
}

type Book struct {
	ID          uuid.UUID
	Title       string
	Authors     []string
	ISBN13      *string
	CoverURL    *string
	Description *string
	PageCount   *int
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Resync status — populated during ResyncAllFromOpenLibrary / ResyncBooks.
	// Nil means the book has never been processed by a resync run.
	OpenLibraryFound *bool
	GoogleBooksFound *bool
	UniCatFound      *bool
	LastResyncAt     *time.Time
}

// HasTag reports whether the user_book has the given tag.
func (ub UserBook) HasTag(tag string) bool {
	for _, t := range ub.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// DisplayTags returns only non-special tags for UI badge rendering.
func (ub UserBook) DisplayTags() []string {
	out := make([]string, 0, len(ub.Tags))
	for _, t := range ub.Tags {
		if !IsSpecialTag(t) {
			out = append(out, t)
		}
	}
	return out
}

type UserBook struct {
	ID              uuid.UUID
	UserID          string
	BookID          uuid.UUID
	Book            *Book
	Status          string
	Tags            []string
	Formats         []string
	ShelfPositions  map[string]int
	Rating          *int16
	FinishedAt      []time.Time
	ProgressMode    string
	CurrentPage     int
	ProgressPercent int
	AddedAt         time.Time
	UpdatedAt       time.Time
}

// DisplayProgressPercent returns the reading progress as a 0-100 percentage. In
// percent mode the stored percent is authoritative; in pages mode it is derived
// from the current page over the book's total page count. It returns 0 when the
// page count is unknown so callers never divide by zero.
func (ub UserBook) DisplayProgressPercent() int {
	if ub.ProgressMode == ProgressModePercent {
		return clampPercent(ub.ProgressPercent)
	}
	if ub.Book == nil || ub.Book.PageCount == nil || *ub.Book.PageCount <= 0 {
		return 0
	}
	ratio := float64(ub.CurrentPage) / float64(*ub.Book.PageCount)
	pct := int(math.Round(ratio * float64(MaxProgressPercent)))
	return clampPercent(pct)
}

func clampPercent(p int) int {
	if p < 0 {
		return 0
	}
	if p > MaxProgressPercent {
		return MaxProgressPercent
	}
	return p
}

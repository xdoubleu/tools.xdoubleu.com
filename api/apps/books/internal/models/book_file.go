package models

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	FileFormatPDF   = "pdf"
	FileFormatEPUB  = "epub"
	FileFormatKEPUB = "kepub"
)

const (
	FileStatusReady      = "ready"
	FileStatusConverting = "converting"
	FileStatusFailed     = "failed"
)

// KoboSyncBook is a join result used by the Kobo sync routes: a user_book with
// the kobo-sync tag that has a ready KEPUB in R2.
type KoboSyncBook struct {
	BookID            uuid.UUID
	Title             string
	Authors           []string
	StorageKey        string
	Size              int64
	KoboSyncEnabledAt time.Time
	ConverterVersion  int16
	// LastSyncedConverterVersion is the KEPUB converter version last sent to
	// the device for this book, nil when it has never been synced before.
	LastSyncedConverterVersion *int16
	// LastSyncedCoverImageID is the CoverImageId last sent to the device, nil
	// when none was recorded.
	LastSyncedCoverImageID *string
}

type BookFile struct {
	ID               uuid.UUID
	BookID           uuid.UUID
	UserID           string
	Format           string
	StorageKey       string
	SizeBytes        int64
	Checksum         *string
	OriginalFilename *string
	Status           string
	SourceFileID     *uuid.UUID
	ConverterVersion int16
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// LibraryFiles summarizes a book's files for the library view.
type LibraryFiles struct {
	// Ready original formats (pdf, epub), sorted.
	Formats []string
	// Format -> "<file id>:<converter version>" of the ready file served.
	Versions map[string]string
	// Format -> figure analysis of the ready file served, once analysed.
	Figures map[string]FigureStats
}

// FigureStats is a file's figure count over the pages it was measured on
// (estimated for an EPUB); Pages is 0 when the file couldn't be analysed.
type FigureStats struct {
	Figures int
	Pages   int
}

// FigureAnalysisTarget is a stored original file awaiting figure analysis.
type FigureAnalysisTarget struct {
	StorageKey string
	Format     string
}

// figureHeavyDensity is the figures-per-page share from which a book reads
// better as a PDF on a tablet or laptop than as an EPUB on a phone or Kobo.
const figureHeavyDensity = 0.15

// ReadSuggestion recommends a format, and with it a device, from the book's
// figure density: "pdf" for iPad/laptop, "epub" for phone/Kobo.
type ReadSuggestion struct {
	Format      string
	FigureCount int
	Pages       int
}

// ReadSuggestion judges the PDF when the book has one, else the EPUB; nil
// until that file is analysed.
func (f LibraryFiles) ReadSuggestion() *ReadSuggestion {
	format := FileFormatEPUB
	if slices.Contains(f.Formats, FileFormatPDF) {
		format = FileFormatPDF
	} else if !slices.Contains(f.Formats, FileFormatEPUB) {
		return nil
	}
	stats, ok := f.Figures[format]
	if !ok || stats.Pages <= 0 {
		return nil
	}
	suggested := FileFormatEPUB
	if float64(stats.Figures) >= figureHeavyDensity*float64(stats.Pages) {
		suggested = FileFormatPDF
	}
	return &ReadSuggestion{
		Format:      suggested,
		FigureCount: stats.Figures,
		Pages:       stats.Pages,
	}
}

package models

import (
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
// the kobo-sync tag that has a ready file in R2. Format is either "kepub" or
// "pdf" depending on the user's per-book kobo-format-pdf tag.
type KoboSyncBook struct {
	BookID     uuid.UUID
	Title      string
	Authors    []string
	Format     string
	StorageKey string
	Size       int64
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
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

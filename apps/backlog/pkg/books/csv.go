package books

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/backlog/internal/models"
)

const (
	goodreadsDateFormat = "2006/01/02"
	isbn13Len           = 13
	isbn10Len           = 10
)

// ParsedEntry holds the extracted data from one row of a Goodreads CSV export.
type ParsedEntry struct {
	Book     models.Book
	UserBook models.UserBook
}

// ParseCSV parses a Goodreads library export CSV into a slice of ParsedEntry.
// Column order is detected from the header row, so it is resilient to reordering.
func ParseCSV(r io.Reader) ([]ParsedEntry, error) {
	reader := csv.NewReader(r)

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}

	idx := buildIndex(header)

	required := []string{"Book Id", "Title", "Author", "Exclusive Shelf"}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return nil, fmt.Errorf("missing required column: %q", col)
		}
	}

	var entries []ParsedEntry
	for {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("reading CSV row: %w", readErr)
		}

		entry, parseErr := parseRow(row, idx)
		if parseErr != nil {
			continue // skip unparseable rows
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func buildIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.TrimSpace(col)] = i
	}
	return idx
}

//nolint:gocognit // many optional CSV fields with independent nil guards
func parseRow(row []string, idx map[string]int) (ParsedEntry, error) {
	goodreadsID := get(row, idx, "Book Id")
	if goodreadsID == "" {
		return ParsedEntry{}, fmt.Errorf(
			"empty Book Id",
		)
	}
	if _, err := strconv.ParseInt(goodreadsID, 10, 64); err != nil {
		return ParsedEntry{}, fmt.Errorf(
			"invalid Book Id %q",
			goodreadsID,
		)
	}

	title := get(row, idx, "Title")
	author := get(row, idx, "Author")

	var isbn13, isbn10 *string
	if v := get(row, idx, "ISBN13"); v != "" && v != `=""` {
		clean := strings.Trim(v, `="`)
		if len(clean) == isbn13Len {
			isbn13 = &clean
		}
	}
	if v := get(row, idx, "ISBN"); v != "" && v != `=""` {
		clean := strings.Trim(v, `="`)
		if len(clean) == isbn10Len {
			isbn10 = &clean
		}
	}

	status := shelfToStatus(get(row, idx, "Exclusive Shelf"))

	var rating *int16
	if v := get(row, idx, "My Rating"); v != "" && v != "0" {
		if n, err := strconv.ParseInt(v, 10, 16); err == nil && n > 0 {
			r := int16(n)
			rating = &r
		}
	}

	var finishedAt []time.Time
	if v := get(row, idx, "Date Read"); v != "" {
		if t, err := time.Parse(goodreadsDateFormat, v); err == nil {
			finishedAt = []time.Time{t}
		}
	}

	var addedAt time.Time
	if v := get(row, idx, "Date Added"); v != "" {
		if t, err := time.Parse(goodreadsDateFormat, v); err == nil {
			addedAt = t
		}
	}

	tags, shelfPositions := parseShelvesWithPositions(
		get(row, idx, "Bookshelves with positions"),
		status,
	)

	book := models.Book{ //nolint:exhaustruct //optional fields
		Title:   title,
		Authors: []string{author},
		ISBN13:  isbn13,
		ISBN10:  isbn10,
		ExternalRefs: map[string]string{
			"goodreads": goodreadsID,
		},
	}

	userBook := models.UserBook{ //nolint:exhaustruct //IDs assigned later
		Status:         status,
		Tags:           tags,
		ShelfPositions: shelfPositions,
		Rating:         rating,
		FinishedAt:     finishedAt,
		AddedAt:        addedAt,
	}

	return ParsedEntry{Book: book, UserBook: userBook}, nil
}

// parseShelvesWithPositions parses the "Bookshelves with positions" CSV column.
// Format: "to-read (#3), technical (#1), own-physical"
// The exclusive shelf (status) is excluded from tags; others become tags.
// Returns tags (non-exclusive shelves) and a position map for all named shelves.
func parseShelvesWithPositions(raw, exclusiveShelf string) ([]string, map[string]int) {
	positions := map[string]int{}
	var tags []string

	if raw == "" {
		return tags, positions
	}

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name, pos := parseShelfEntry(part)
		if name == "" {
			continue
		}

		if pos > 0 {
			positions[name] = pos
		}

		if name != exclusiveShelf {
			tags = append(tags, name)
		}
	}

	return tags, positions
}

// parseShelfEntry splits "shelf-name (#42)" into ("shelf-name", 42).
// Returns ("", 0) for empty input.
func parseShelfEntry(s string) (string, int) {
	open := strings.LastIndex(s, "(#")
	if open < 0 {
		return strings.TrimSpace(s), 0
	}

	name := strings.TrimSpace(s[:open])
	rest := s[open+2:]
	closeParen := strings.Index(rest, ")")
	if closeParen < 0 {
		return name, 0
	}

	pos, err := strconv.Atoi(strings.TrimSpace(rest[:closeParen]))
	if err != nil || pos < 0 {
		return name, 0
	}

	return name, pos
}

func shelfToStatus(shelf string) string {
	if shelf == "" {
		return models.StatusToRead
	}
	return shelf
}

func get(row []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

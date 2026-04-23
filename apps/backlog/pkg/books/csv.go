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

const goodreadsDateFormat = "2006/01/02"

// ParsedEntry holds the extracted data from one row of a Goodreads CSV export.
type ParsedEntry struct {
	Book   models.Book
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
			return nil, fmt.Errorf("CSV missing required column %q", col)
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

func parseRow(row []string, idx map[string]int) (ParsedEntry, error) {
	goodreadsID := get(row, idx, "Book Id")
	if goodreadsID == "" {
		return ParsedEntry{}, fmt.Errorf("empty Book Id") //nolint:exhaustruct //zero value
	}
	if _, err := strconv.ParseInt(goodreadsID, 10, 64); err != nil {
		return ParsedEntry{}, fmt.Errorf("invalid Book Id %q", goodreadsID) //nolint:exhaustruct //zero value
	}

	title := get(row, idx, "Title")
	author := get(row, idx, "Author")

	var isbn13, isbn10 *string
	if v := get(row, idx, "ISBN13"); v != "" && v != `=""` {
		clean := strings.Trim(v, `="`)
		if len(clean) == 13 {
			isbn13 = &clean
		}
	}
	if v := get(row, idx, "ISBN"); v != "" && v != `=""` {
		clean := strings.Trim(v, `="`)
		if len(clean) == 10 {
			isbn10 = &clean
		}
	}

	shelf := shelfToStatus(get(row, idx, "Exclusive Shelf"))

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
		Status:     shelf,
		Rating:     rating,
		FinishedAt: finishedAt,
	}

	return ParsedEntry{Book: book, UserBook: userBook}, nil
}

func shelfToStatus(shelf string) string {
	switch shelf {
	case "read":
		return models.StatusFinished
	case "currently-reading":
		return models.StatusReading
	default:
		return models.StatusWishlist
	}
}

func get(row []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

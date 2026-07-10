//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
)

// fakeOLClient is a configurable openlibrary.Client stub for enrichment tests.
// All fields other than mu are set once before the test and read-only during
// the test, except calls which is protected by mu so the stub is safe for
// concurrent use (e.g. from the parallel resync fan-out).
type fakeOLClient struct {
	detail *openlibrary.ExternalBook
	err    error
	mu     sync.Mutex
	calls  int
}

func (f *fakeOLClient) Search(
	_ context.Context,
	_ string,
) ([]openlibrary.ExternalBook, error) {
	return nil, nil
}

func (f *fakeOLClient) Get(
	_ context.Context,
	_ string,
) (*openlibrary.ExternalBook, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.detail, f.err
}

func (f *fakeOLClient) GetByISBN(
	_ context.Context,
	_ string,
) (*openlibrary.ExternalBook, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.detail, f.err
}

func (f *fakeOLClient) FetchCover(
	_ context.Context,
	_ string,
) ([]byte, string, error) {
	return nil, "", errors.New("fakeOLClient: FetchCover not implemented")
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestCountDatesOn(t *testing.T) {
	dates := []time.Time{
		time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC),
	}
	assert.Equal(t, 2, countDatesOn(dates, "2024-01-15"))
	assert.Equal(t, 1, countDatesOn(dates, "2024-03-10"))
	assert.Equal(t, 0, countDatesOn(dates, "2024-06-01"))
}

func TestCountDatesOn_Empty(t *testing.T) {
	assert.Equal(t, 0, countDatesOn(nil, "2024-01-15"))
}

func TestExternalToBook(t *testing.T) {
	isbn13 := "9780140449112"
	cover := "https://example.com/cover.jpg"
	desc := "A classic."
	pages := 496

	ext := openlibrary.ExternalBook{
		Provider:    "openlibrary",
		ProviderID:  "OL42W",
		Title:       "The Odyssey",
		Authors:     []string{"Homer"},
		ISBN13:      &isbn13,
		CoverURL:    &cover,
		Description: &desc,
		PageCount:   &pages,
	}

	book := externalToBook(ext)

	assert.Equal(t, "The Odyssey", book.Title)
	assert.Equal(t, []string{"Homer"}, book.Authors)
	assert.Equal(t, &isbn13, book.ISBN13)
	assert.Equal(t, &cover, book.CoverURL)
	assert.Equal(t, &desc, book.Description)
	assert.Equal(t, &pages, book.PageCount)
}

func TestExternalToBook_FallsBackToOpenLibraryCover(t *testing.T) {
	isbn13 := "9780140449112"
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //optional fields nil
		Provider:   "openlibrary",
		ProviderID: "OL42W",
		Title:      "The Odyssey",
		Authors:    []string{"Homer"},
		ISBN13:     &isbn13,
	}

	book := externalToBook(ext)

	require.NotNil(t, book.CoverURL)
	assert.Equal(t, openlibrary.CoverURLByISBN(&isbn13), *book.CoverURL)
}

func TestGetExternal_DelegatesToOpenLibrary(t *testing.T) {
	book := &openlibrary.ExternalBook{ //nolint:exhaustruct //partial
		Provider:   "openlibrary",
		ProviderID: "OL42W",
		Title:      "The Odyssey",
	}
	fake := &fakeOLClient{detail: book} //nolint:exhaustruct //zero values fine
	svc := &BookService{                //nolint:exhaustruct //only external needed
		logger:   logging.NewNopLogger(),
		external: fake,
	}

	out, err := svc.GetExternal(context.Background(), "openlibrary", "OL42W")
	require.NoError(t, err)
	assert.Equal(t, book, out)
}

func TestGetExternal_UnknownProvider_ReturnsNotFound(t *testing.T) {
	fake := &fakeOLClient{} //nolint:exhaustruct //zero values fine
	svc := &BookService{    //nolint:exhaustruct //only external needed
		logger:   logging.NewNopLogger(),
		external: fake,
	}

	_, err := svc.GetExternal(context.Background(), "googlebooks", "anything")
	require.ErrorIs(t, err, openlibrary.ErrNotFound)
	assert.Zero(t, fake.calls, "unknown provider should not call the client")
}

func TestEnrichByISBN_NoISBN13_ReturnsUnchanged(t *testing.T) {
	fake := &fakeOLClient{} //nolint:exhaustruct //zero values intended
	svc := &BookService{    //nolint:exhaustruct //only external needed
		logger:   logging.NewNopLogger(),
		external: fake,
	}
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //no ISBN13
		Title: "No ISBN",
	}
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, ext, out)
	assert.Zero(t, fake.calls, "no lookup without ISBN13")
}

func TestEnrichByISBN_AlreadyComplete_SkipsLookup(t *testing.T) {
	fake := &fakeOLClient{} //nolint:exhaustruct //zero values intended
	svc := &BookService{    //nolint:exhaustruct //only external needed
		logger:   logging.NewNopLogger(),
		external: fake,
	}
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //all enriched fields set
		Title:       "Complete",
		ISBN13:      strPtr("9780140449112"),
		Description: strPtr("desc"),
		PageCount:   intPtr(100),
		CoverURL:    strPtr("https://example.com/c.jpg"),
	}
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, ext, out)
	assert.Zero(t, fake.calls, "no lookup when all fields present")
}

func TestEnrichByISBN_FillsMissingFields(t *testing.T) {
	fake := &fakeOLClient{ //nolint:exhaustruct //err nil
		detail: &openlibrary.ExternalBook{ //nolint:exhaustruct //only enriched fields
			Description: strPtr("fetched description"),
			PageCount:   intPtr(496),
			CoverURL:    strPtr("https://example.com/fetched.jpg"),
		},
	}
	svc := &BookService{ //nolint:exhaustruct //only external needed
		logger:   logging.NewNopLogger(),
		external: fake,
	}
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //missing fields filled
		Title:  "Sparse",
		ISBN13: strPtr("9780140449112"),
	}
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, 1, fake.calls)
	require.NotNil(t, out.Description)
	assert.Equal(t, "fetched description", *out.Description)
	require.NotNil(t, out.PageCount)
	assert.Equal(t, 496, *out.PageCount)
	require.NotNil(t, out.CoverURL)
	assert.Equal(t, "https://example.com/fetched.jpg", *out.CoverURL)
}

func TestEnrichByISBN_LookupError_ReturnsUnchanged(t *testing.T) {
	fake := &fakeOLClient{ //nolint:exhaustruct //detail nil
		err: errors.New("boom"),
	}
	svc := &BookService{ //nolint:exhaustruct //only external needed
		logger:   logging.NewNopLogger(),
		external: fake,
	}
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //missing fields
		Title:  "Errors",
		ISBN13: strPtr("9780140449112"),
	}
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, 1, fake.calls)
	assert.Nil(t, out.Description)
	assert.Nil(t, out.PageCount)
}

func TestExternalToBook_NilFields(t *testing.T) {
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //optional fields nil
		Provider:   "manual",
		ProviderID: "1",
		Title:      "Untitled",
		Authors:    []string{},
	}

	book := externalToBook(ext)

	assert.Equal(t, "Untitled", book.Title)
	assert.Nil(t, book.ISBN13)
	assert.Nil(t, book.CoverURL)
	assert.Nil(t, book.Description)
}

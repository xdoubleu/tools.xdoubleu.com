//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
)

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
	ext := SourceProposal{ //nolint:exhaustruct //Index/Differs unused
		Source:      "hardcover",
		Title:       "The Odyssey",
		Authors:     []string{"Homer"},
		ISBN13:      "9780140449112",
		CoverURL:    "https://example.com/cover.jpg",
		Description: "A classic.",
		PageCount:   496,
	}

	book := externalToBook(ext)

	assert.Equal(t, "The Odyssey", book.Title)
	assert.Equal(t, []string{"Homer"}, book.Authors)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, "9780140449112", *book.ISBN13)
	require.NotNil(t, book.CoverURL)
	assert.Equal(t, "https://example.com/cover.jpg", *book.CoverURL)
	require.NotNil(t, book.Description)
	assert.Equal(t, "A classic.", *book.Description)
	require.NotNil(t, book.PageCount)
	assert.Equal(t, 496, *book.PageCount)
	require.NotNil(t, book.MetadataSource)
	assert.Equal(t, "hardcover", *book.MetadataSource)
}

// TestExternalToBook_NoCoverStaysNil verifies a proposal with no cover URL
// leaves the book's CoverURL nil — there is no ISBN-keyed cover fallback
// anymore (that was Open Library's covers.openlibrary.org, now removed).
func TestExternalToBook_NoCoverStaysNil(t *testing.T) {
	ext := SourceProposal{ //nolint:exhaustruct //optional fields intentionally empty
		Source: "unicat",
		Title:  "The Odyssey",
		ISBN13: "9780140449112",
	}

	book := externalToBook(ext)

	assert.Nil(t, book.CoverURL)
}

func TestGetExternal_DelegatesToHardcover(t *testing.T) {
	//nolint:exhaustruct //partial
	book := &hardcover.ExternalBook{Title: "The Odyssey"}
	fake := &fakeHCClient{byISBN: book} //nolint:exhaustruct //zero values fine
	svc := &BookService{                //nolint:exhaustruct //only hardcover needed
		logger:    logging.NewNopLogger(),
		hardcover: fake,
	}

	out, err := svc.GetExternal(context.Background(), "hardcover", "9780140449112")
	require.NoError(t, err)
	assert.Equal(t, "The Odyssey", out.Title)
	assert.Equal(t, "hardcover", out.Source)
}

func TestGetExternal_DelegatesToUniCat(t *testing.T) {
	//nolint:exhaustruct //partial
	book := &unicat.ExternalBook{Title: "The Odyssey"}
	fake := &fakeUCClient{byISBN: book} //nolint:exhaustruct //zero values fine
	svc := &BookService{                //nolint:exhaustruct //only uniCat needed
		logger: logging.NewNopLogger(),
		uniCat: fake,
	}

	out, err := svc.GetExternal(context.Background(), "unicat", "9780140449112")
	require.NoError(t, err)
	assert.Equal(t, "The Odyssey", out.Title)
	assert.Equal(t, "unicat", out.Source)
}

func TestGetExternal_HardcoverError_Propagates(t *testing.T) {
	fake := &fakeHCClient{err: errors.New("boom")} //nolint:exhaustruct //partial
	//nolint:exhaustruct //only hardcover needed
	svc := &BookService{
		logger:    logging.NewNopLogger(),
		hardcover: fake,
	}

	_, err := svc.GetExternal(context.Background(), "hardcover", "9780140449112")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrExternalNotFound)
}

func TestGetExternal_UniCatNotFound_ReturnsNotFound(t *testing.T) {
	fake := &fakeUCClient{} //nolint:exhaustruct //byISBN nil -> ErrNotFound
	svc := &BookService{    //nolint:exhaustruct //only uniCat needed
		logger: logging.NewNopLogger(),
		uniCat: fake,
	}

	_, err := svc.GetExternal(context.Background(), "unicat", "9780140449112")
	require.ErrorIs(t, err, ErrExternalNotFound)
}

func TestGetExternal_UniCatProviderNotConfigured_ReturnsNotFound(t *testing.T) {
	//nolint:exhaustruct //only logger needed
	svc := &BookService{logger: logging.NewNopLogger()}

	_, err := svc.GetExternal(context.Background(), "unicat", "anything")
	require.ErrorIs(t, err, ErrExternalNotFound)
}

func TestGetExternal_ProviderNotConfigured_ReturnsNotFound(t *testing.T) {
	svc := &BookService{ //nolint:exhaustruct //only logger needed
		logger: logging.NewNopLogger(),
	}

	_, err := svc.GetExternal(context.Background(), "hardcover", "anything")
	require.ErrorIs(t, err, ErrExternalNotFound)
}

func TestGetExternal_UnknownProvider_ReturnsNotFound(t *testing.T) {
	svc := &BookService{ //nolint:exhaustruct //only logger needed
		logger: logging.NewNopLogger(),
	}

	_, err := svc.GetExternal(context.Background(), "unknownprovider", "anything")
	require.ErrorIs(t, err, ErrExternalNotFound)
}

func TestEnrichByISBN_NoISBN13_ReturnsUnchanged(t *testing.T) {
	fake := &fakeHCClient{} //nolint:exhaustruct //zero values intended
	svc := &BookService{    //nolint:exhaustruct //only hardcover needed
		logger:    logging.NewNopLogger(),
		hardcover: fake,
	}
	ext := SourceProposal{Title: "No ISBN"} //nolint:exhaustruct //no ISBN13
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, ext, out)
	assert.Zero(t, fake.calls, "no lookup without ISBN13")
}

func TestEnrichByISBN_AlreadyComplete_SkipsLookup(t *testing.T) {
	fake := &fakeHCClient{} //nolint:exhaustruct //zero values intended
	svc := &BookService{    //nolint:exhaustruct //only hardcover needed
		logger:    logging.NewNopLogger(),
		hardcover: fake,
	}
	ext := SourceProposal{ //nolint:exhaustruct //all enriched fields set
		Title:       "Complete",
		ISBN13:      "9780140449112",
		Description: "desc",
		PageCount:   100,
		CoverURL:    "https://example.com/c.jpg",
	}
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, ext, out)
	assert.Zero(t, fake.calls, "no lookup when all fields present")
}

func TestEnrichByISBN_FillsMissingFieldsFromHardcover(t *testing.T) {
	cover := "https://example.com/fetched.jpg"
	fake := &fakeHCClient{ //nolint:exhaustruct //err nil
		byISBN: &hardcover.ExternalBook{ //nolint:exhaustruct //only enriched fields
			Description: strPtr("fetched description"),
			PageCount:   intPtr(496),
			CoverURL:    &cover,
		},
	}
	svc := &BookService{ //nolint:exhaustruct //only hardcover needed
		logger:    logging.NewNopLogger(),
		hardcover: fake,
	}
	ext := SourceProposal{ //nolint:exhaustruct //missing fields filled
		Title:  "Sparse",
		ISBN13: "9780140449112",
	}
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, 1, fake.calls)
	assert.Equal(t, "fetched description", out.Description)
	assert.Equal(t, 496, out.PageCount)
	assert.Equal(t, "https://example.com/fetched.jpg", out.CoverURL)
}

// TestEnrichByISBN_FillsRemainingFromUniCat verifies that when Hardcover
// fills some but not all fields, UniCat (which has no cover) is consulted
// for the still-missing description/page count.
func TestEnrichByISBN_FillsRemainingFromUniCat(t *testing.T) {
	hcFake := &fakeHCClient{ //nolint:exhaustruct //only cover supplied
		byISBN: &hardcover.ExternalBook{ //nolint:exhaustruct //only cover
			CoverURL: strPtr("https://hardcover.app/c.jpg"),
		},
	}
	ucFake := &fakeUCClient{ //nolint:exhaustruct //err nil
		byISBN: &unicat.ExternalBook{ //nolint:exhaustruct //only enriched fields
			Description: strPtr("fetched from unicat"),
			PageCount:   intPtr(321),
		},
	}
	svc := &BookService{ //nolint:exhaustruct //only hardcover/uniCat needed
		logger:    logging.NewNopLogger(),
		hardcover: hcFake,
		uniCat:    ucFake,
	}
	ext := SourceProposal{ //nolint:exhaustruct //missing fields filled
		Title:  "Sparse",
		ISBN13: "9780140449112",
	}
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, 1, hcFake.calls)
	assert.Equal(t, 1, ucFake.calls)
	assert.Equal(t, "https://hardcover.app/c.jpg", out.CoverURL)
	assert.Equal(t, "fetched from unicat", out.Description)
	assert.Equal(t, 321, out.PageCount)
}

func TestEnrichByISBN_LookupError_ReturnsUnchanged(t *testing.T) {
	fake := &fakeHCClient{ //nolint:exhaustruct //byISBN nil
		err: errors.New("boom"),
	}
	svc := &BookService{ //nolint:exhaustruct //only hardcover needed
		logger:    logging.NewNopLogger(),
		hardcover: fake,
	}
	ext := SourceProposal{ //nolint:exhaustruct //missing fields
		Title:  "Errors",
		ISBN13: "9780140449112",
	}
	out := svc.enrichByISBN(context.Background(), ext)
	assert.Equal(t, 1, fake.calls)
	assert.Empty(t, out.Description)
	assert.Zero(t, out.PageCount)
}

func TestExternalToBook_ManualSource_NoProvenance(t *testing.T) {
	ext := SourceProposal{ //nolint:exhaustruct //optional fields nil
		Source:  "manual",
		Title:   "Untitled",
		Authors: []string{},
	}

	book := externalToBook(ext)

	assert.Equal(t, "Untitled", book.Title)
	assert.Nil(t, book.ISBN13)
	assert.Nil(t, book.CoverURL)
	assert.Nil(t, book.Description)
	assert.Nil(t, book.MetadataSource, "manual entries must not claim provenance")
}

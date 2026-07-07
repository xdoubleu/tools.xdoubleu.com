package books_test

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// goodreadsCSVHeader is the header row shared by the hand-built CSVs below;
// pair with csvRow to build a minimal single-book export.
const goodreadsCSVHeader = "Book Id,Title,Author,ISBN,ISBN13,My Rating," +
	"Exclusive Shelf,Bookshelves with positions,Date Read\n"

func csvRow(bookIDNum int, title, author, isbn13, shelf string) string {
	return fmt.Sprintf(
		`%d,%s,%s,,"=""%s""",0,%s,,`+"\n",
		bookIDNum, title, author, isbn13, shelf,
	)
}

// csvRowWithBookshelves is csvRow plus the "Bookshelves with positions"
// column, needed to exercise tag parsing (e.g. "read (#1), technical (#2)").
// bookshelves is quoted since its value contains commas.
func csvRowWithBookshelves(
	bookIDNum int,
	title, author, isbn13, shelf, bookshelves string,
) string {
	return fmt.Sprintf(
		`%d,%s,%s,,"=""%s""",0,%s,"%s",`+"\n",
		bookIDNum, title, author, isbn13, shelf, bookshelves,
	)
}

// findMismatch returns the first mismatch tagged with the given difference,
// failing the test if none is found.
func findMismatch(
	t *testing.T,
	mismatches []*booksv1.BookMismatch,
	difference string,
) *booksv1.BookMismatch {
	t.Helper()
	for _, m := range mismatches {
		for _, d := range m.Differences {
			if d == difference {
				return m
			}
		}
	}
	require.Failf(t, "mismatch not found", "no mismatch tagged %q", difference)
	return nil
}

func TestConnectImportBooks(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.ImportBooksRequest{
		CsvData: []byte(goodreadsCSVForImport),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ImportBooks(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
	assert.GreaterOrEqual(t, resp.Msg.ImportedCount, int32(0))
}

func TestConnectCompareCSV(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.CompareCSVRequest{
		CsvData: []byte(goodreadsCSVForImport),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.CompareCSV(ctx, req)
	assert.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Msg)
	assert.GreaterOrEqual(t, resp.Msg.CsvCount, int32(0))
	assert.GreaterOrEqual(t, resp.Msg.LibraryCount, int32(0))
}

func TestConnectApplyCSVFix_Unauthenticated(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ApplyCSVFixRequest{
		CsvData:    []byte(goodreadsCSVForImport),
		MismatchId: "csv:0",
		Difference: "missing-in-library",
	})

	// No cookie: the AppAccess middleware redirects before the handler's own
	// unauthenticated check runs, and the Connect client surfaces that
	// redirect as CodeNotFound (matches every other app-access-gated RPC).
	_, err := client.ApplyCSVFix(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestConnectApplyCSVFix_UnknownMismatch_NotFound(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.ApplyCSVFixRequest{
		CsvData:    []byte(goodreadsCSVForImport),
		MismatchId: "does-not-exist",
		Difference: "missing-in-library",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ApplyCSVFix(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestConnectApplyCSVFix_MissingInLibrary_Adds(t *testing.T) {
	client := newBooksTestClient(t)
	isbn := isbnFromUUID(uuid.New())
	// Title embeds the ISBN so repeated test runs against a persistent DB
	// never collide with a book added by a previous run.
	title := "Fix Add Test " + isbn
	csvData := goodreadsCSVHeader + csvRow(90001, title, "Fix Author", isbn, "to-read")

	compareReq := connect.NewRequest(
		&booksv1.CompareCSVRequest{CsvData: []byte(csvData)},
	)
	compareReq.Header().Set("Cookie", accessToken.String())
	compareResp, err := client.CompareCSV(context.Background(), compareReq)
	require.NoError(t, err)
	m := findMismatch(t, compareResp.Msg.Mismatches, "missing-in-library")

	fixReq := connect.NewRequest(&booksv1.ApplyCSVFixRequest{
		CsvData:    []byte(csvData),
		MismatchId: m.Id,
		Difference: "missing-in-library",
	})
	fixReq.Header().Set("Cookie", accessToken.String())
	_, err = client.ApplyCSVFix(context.Background(), fixReq)
	require.NoError(t, err)

	lib, err := testApp.Services.Books.GetLibrary(context.Background(), userID)
	require.NoError(t, err)
	found := false
	for _, ub := range lib {
		if ub.Book != nil && ub.Book.ISBN13 != nil && *ub.Book.ISBN13 == isbn {
			found = true
			assert.Equal(t, models.StatusToRead, ub.Status)
		}
	}
	assert.True(t, found, "expected book to be added to library")
}

func TestConnectApplyCSVFix_StatusFix_UpdatesStatus(t *testing.T) {
	client := newBooksTestClient(t)
	isbn := isbnFromUUID(uuid.New())

	importCSV := goodreadsCSVHeader + csvRow(
		90002,
		"Fix Status Test",
		"Fix Author",
		isbn,
		"read",
	)
	importReq := connect.NewRequest(
		&booksv1.ImportBooksRequest{CsvData: []byte(importCSV)},
	)
	importReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ImportBooks(context.Background(), importReq)
	require.NoError(t, err)

	compareCSV := goodreadsCSVHeader +
		csvRow(90002, "Fix Status Test", "Fix Author", isbn, "to-read")
	compareReq := connect.NewRequest(
		&booksv1.CompareCSVRequest{CsvData: []byte(compareCSV)},
	)
	compareReq.Header().Set("Cookie", accessToken.String())
	compareResp, err := client.CompareCSV(context.Background(), compareReq)
	require.NoError(t, err)
	m := findMismatch(t, compareResp.Msg.Mismatches, "status")

	fixReq := connect.NewRequest(&booksv1.ApplyCSVFixRequest{
		CsvData:    []byte(compareCSV),
		MismatchId: m.Id,
		Difference: "status",
	})
	fixReq.Header().Set("Cookie", accessToken.String())
	_, err = client.ApplyCSVFix(context.Background(), fixReq)
	require.NoError(t, err)

	lib, err := testApp.Services.Books.GetLibrary(context.Background(), userID)
	require.NoError(t, err)
	found := false
	for _, ub := range lib {
		if ub.Book != nil && ub.Book.ISBN13 != nil && *ub.Book.ISBN13 == isbn {
			found = true
			assert.Equal(t, models.StatusToRead, ub.Status)
		}
	}
	assert.True(t, found, "expected book to remain in library with updated status")
}

func TestConnectApplyCSVFix_ISBNFix_UpdatesISBN(t *testing.T) {
	client := newBooksTestClient(t)
	oldISBN := isbnFromUUID(uuid.New())
	newISBN := isbnFromUUID(uuid.New())

	importCSV := goodreadsCSVHeader +
		csvRow(90003, "Fix ISBN Test", "Fix ISBN Author", oldISBN, "read")
	importReq := connect.NewRequest(
		&booksv1.ImportBooksRequest{CsvData: []byte(importCSV)},
	)
	importReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ImportBooks(context.Background(), importReq)
	require.NoError(t, err)

	compareCSV := goodreadsCSVHeader +
		csvRow(90003, "Fix ISBN Test", "Fix ISBN Author", newISBN, "read")
	compareReq := connect.NewRequest(
		&booksv1.CompareCSVRequest{CsvData: []byte(compareCSV)},
	)
	compareReq.Header().Set("Cookie", accessToken.String())
	compareResp, err := client.CompareCSV(context.Background(), compareReq)
	require.NoError(t, err)
	m := findMismatch(t, compareResp.Msg.Mismatches, "isbn")

	fixReq := connect.NewRequest(&booksv1.ApplyCSVFixRequest{
		CsvData:    []byte(compareCSV),
		MismatchId: m.Id,
		Difference: "isbn",
	})
	fixReq.Header().Set("Cookie", accessToken.String())
	_, err = client.ApplyCSVFix(context.Background(), fixReq)
	require.NoError(t, err)

	lib, err := testApp.Services.Books.GetLibrary(context.Background(), userID)
	require.NoError(t, err)
	found := false
	for _, ub := range lib {
		if ub.Book != nil && ub.Book.ISBN13 != nil && *ub.Book.ISBN13 == newISBN {
			found = true
		}
	}
	assert.True(t, found, "expected catalog ISBN to be updated to the CSV value")
}

func TestConnectApplyCSVFix_TitleFix_UpdatesTitle(t *testing.T) {
	client := newBooksTestClient(t)
	isbn := isbnFromUUID(uuid.New())

	importCSV := goodreadsCSVHeader + csvRow(
		90004,
		"Old Title",
		"Fix Title Author",
		isbn,
		"read",
	)
	importReq := connect.NewRequest(
		&booksv1.ImportBooksRequest{CsvData: []byte(importCSV)},
	)
	importReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ImportBooks(context.Background(), importReq)
	require.NoError(t, err)

	compareCSV := goodreadsCSVHeader + csvRow(
		90004,
		"New Title",
		"Fix Title Author",
		isbn,
		"read",
	)
	compareReq := connect.NewRequest(
		&booksv1.CompareCSVRequest{CsvData: []byte(compareCSV)},
	)
	compareReq.Header().Set("Cookie", accessToken.String())
	compareResp, err := client.CompareCSV(context.Background(), compareReq)
	require.NoError(t, err)
	m := findMismatch(t, compareResp.Msg.Mismatches, "title")

	fixReq := connect.NewRequest(&booksv1.ApplyCSVFixRequest{
		CsvData:    []byte(compareCSV),
		MismatchId: m.Id,
		Difference: "title",
	})
	fixReq.Header().Set("Cookie", accessToken.String())
	_, err = client.ApplyCSVFix(context.Background(), fixReq)
	require.NoError(t, err)

	lib, err := testApp.Services.Books.GetLibrary(context.Background(), userID)
	require.NoError(t, err)
	found := false
	for _, ub := range lib {
		if ub.Book != nil && ub.Book.ISBN13 != nil && *ub.Book.ISBN13 == isbn {
			found = true
			assert.Equal(t, "New Title", ub.Book.Title)
		}
	}
	assert.True(t, found, "expected catalog title to be updated to the CSV value")
}

func TestConnectApplyCSVFix_TagsFix_UpdatesTags(t *testing.T) {
	client := newBooksTestClient(t)
	isbn := isbnFromUUID(uuid.New())

	importCSV := goodreadsCSVHeader + csvRowWithBookshelves(
		90005,
		"Fix Tags Test",
		"Fix Tags Author",
		isbn,
		"read",
		"read (#1), technical (#2), own-physical (#3)",
	)
	importReq := connect.NewRequest(
		&booksv1.ImportBooksRequest{CsvData: []byte(importCSV)},
	)
	importReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ImportBooks(context.Background(), importReq)
	require.NoError(t, err)

	// CSV drops "own-physical" — the fix should replace the library's tags
	// with just what the CSV has, not merge.
	compareCSV := goodreadsCSVHeader + csvRowWithBookshelves(
		90005,
		"Fix Tags Test",
		"Fix Tags Author",
		isbn,
		"read",
		"read (#1), technical (#2)",
	)
	compareReq := connect.NewRequest(
		&booksv1.CompareCSVRequest{CsvData: []byte(compareCSV)},
	)
	compareReq.Header().Set("Cookie", accessToken.String())
	compareResp, err := client.CompareCSV(context.Background(), compareReq)
	require.NoError(t, err)
	m := findMismatch(t, compareResp.Msg.Mismatches, "tags")

	fixReq := connect.NewRequest(&booksv1.ApplyCSVFixRequest{
		CsvData:    []byte(compareCSV),
		MismatchId: m.Id,
		Difference: "tags",
	})
	fixReq.Header().Set("Cookie", accessToken.String())
	_, err = client.ApplyCSVFix(context.Background(), fixReq)
	require.NoError(t, err)

	lib, err := testApp.Services.Books.GetLibrary(context.Background(), userID)
	require.NoError(t, err)
	found := false
	for _, ub := range lib {
		if ub.Book != nil && ub.Book.ISBN13 != nil && *ub.Book.ISBN13 == isbn {
			found = true
			assert.Equal(t, []string{"technical"}, ub.Tags)
		}
	}
	assert.True(t, found, "expected library tags to be replaced with the CSV's tags")
}

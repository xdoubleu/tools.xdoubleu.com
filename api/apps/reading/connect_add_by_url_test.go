package reading_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/reading/internal/models"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// articlePageHTML builds an HTML page with enough content for readability
// extraction to accept it.
func articlePageHTML(title string) string {
	const para = `<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit,
sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad
minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit
esse cillum dolore eu fugiat nulla pariatur.</p>`
	return `<html><head><title>` + title + `</title></head><body><article><h1>` +
		title + `</h1>` + para + para + para + `</article></body></html>`
}

// fakePDFBytes is a minimal blob passing the %PDF magic-byte check.
func fakePDFBytes() []byte {
	return []byte("%PDF-1.4 fake test pdf content")
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err)
	return id
}

func addByURL(
	t *testing.T,
	url, category string,
) (*readingv1.AddBookByURLResponse, error) {
	t.Helper()
	client := newBooksTestClient(t)
	req := connect.NewRequest(&readingv1.AddBookByURLRequest{
		Url:      url,
		Category: category,
	})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.AddBookByURL(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func TestAddBookByURL_Article(t *testing.T) {
	url := "https://blog.example.com/posts/" + uuid.NewString() +
		"/why-tests-matter"
	mockWebFetch.SetHTML(url, articlePageHTML("Why Tests Matter"))

	msg, err := addByURL(t, url, "")
	require.NoError(t, err)
	require.NotNil(t, msg.UserBook.Book)
	assert.False(t, msg.AlreadyInLibrary)
	assert.Equal(t, models.CategoryArticle, msg.UserBook.Book.Category)
	assert.Equal(t, url, msg.UserBook.Book.SourceUrl)
	assert.Contains(t, msg.UserBook.Book.Title, "Why Tests Matter")

	// The built EPUB must be stored and ready.
	statusResult, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, mustUUID(t, msg.UserBook.BookId),
	)
	require.NoError(t, err)
	assert.True(t, statusResult.HasEPUB)

	// Pasting the same URL again reports already-in-library.
	again, err := addByURL(t, url, "")
	require.NoError(t, err)
	assert.True(t, again.AlreadyInLibrary)
	assert.Equal(t, msg.UserBook.BookId, again.UserBook.BookId)
}

func TestAddBookByURL_DirectPDFLink(t *testing.T) {
	url := "https://example.com/whitepapers/" + uuid.NewString() +
		"/consensus.pdf"
	mockWebFetch.SetBody(url, "application/pdf", fakePDFBytes())

	msg, err := addByURL(t, url, "")
	require.NoError(t, err)
	assert.Equal(t, models.CategoryArticle, msg.UserBook.Book.Category)
	// Fallback title comes from the URL's last path segment.
	assert.Equal(t, "consensus", msg.UserBook.Book.Title)
}

func TestAddBookByURL_PaperOverrideOnPlainPDF(t *testing.T) {
	url := "https://example.com/papers/" + uuid.NewString() +
		"/quantum-methods.pdf"
	mockWebFetch.SetBody(url, "application/pdf", fakePDFBytes())

	msg, err := addByURL(t, url, models.CategoryPaper)
	require.NoError(t, err)
	assert.Equal(t, models.CategoryPaper, msg.UserBook.Book.Category)
}

func TestAddBookByURL_Errors(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		category string
		wantCode connect.Code
	}{
		{"empty url", "", "", connect.CodeInvalidArgument},
		{"bad scheme", "ftp://example.com/x", "", connect.CodeInvalidArgument},
		{"bad category", "https://example.com/x", "rss",
			connect.CodeInvalidArgument},
		{"unreachable page", "https://gone.example.com/404", "",
			connect.CodeUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := addByURL(t, tt.url, tt.category)
			require.Error(t, err)
			assert.Equal(t, tt.wantCode, connect.CodeOf(err))
		})
	}
}

func TestAddBookByURL_NonHTMLNonPDF(t *testing.T) {
	url := "https://example.com/" + uuid.NewString() + "/archive.zip"
	mockWebFetch.SetBody(url, "application/zip", []byte("PK\x03\x04zip"))

	_, err := addByURL(t, url, "")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

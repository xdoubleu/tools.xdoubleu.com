//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/internal/mocks"
	"tools.xdoubleu.com/apps/reading/internal/models"
)

func TestFeedItemHTML(t *testing.T) {
	//nolint:exhaustruct // gofeed.Item fields left zero are irrelevant here
	tests := []struct {
		name string
		item *gofeed.Item
		want string
	}{
		{
			"prefers item.Content when present",
			&gofeed.Item{Content: "<p>full</p>"},
			"<p>full</p>",
		},
		{
			"falls back to non-namespaced content:encoded",
			&gofeed.Item{
				Custom: map[string]string{"encoded": "<p>encoded</p>"},
			},
			"<p>encoded</p>",
		},
		{
			"empty when neither is set",
			&gofeed.Item{},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, feedItemHTML(tt.item))
		})
	}
}

// newTestFeedService builds a FeedService with only the fields
// enrichFromLinkedPage touches — its DB-backed collaborators are untested here.
func newTestFeedService(webFetch *mocks.MockWebFetchClient) *FeedService {
	//nolint:exhaustruct // feeds/ingest/books/conversion unused by this method
	return &FeedService{
		logger:   logging.NewNopLogger(),
		webFetch: webFetch,
	}
}

// newTestArticleContent builds an ArticleContent with only SourceURL set —
// the rest are populated (or not) by the method under test.
func newTestArticleContent(sourceURL string) *ArticleContent {
	//nolint:exhaustruct // remaining fields are what enrichFromLinkedPage sets
	return &ArticleContent{SourceURL: sourceURL}
}

func TestEnrichFromLinkedPage(t *testing.T) {
	t.Run("populates content on success", func(t *testing.T) {
		mock := mocks.NewMockWebFetchClient()
		mock.SetHTML("https://blog.example.com/post", `<html><head>
<title>My Post</title></head><body><article><h1>My Post</h1>
<p>`+loremParagraph+`</p><p>`+loremParagraph+`</p></article></body></html>`)
		s := newTestFeedService(mock)

		content := newTestArticleContent("https://blog.example.com/post")
		s.enrichFromLinkedPage(context.Background(), content)

		assert.NotEmpty(t, content.HTML)
	})

	t.Run("leaves content empty on fetch error", func(t *testing.T) {
		s := newTestFeedService(mocks.NewMockWebFetchClient())

		content := newTestArticleContent("https://example.com/missing")
		s.enrichFromLinkedPage(context.Background(), content)

		assert.Empty(t, content.HTML)
	})

	t.Run("leaves content empty on non-HTML response", func(t *testing.T) {
		mock := mocks.NewMockWebFetchClient()
		mock.SetBody("https://example.com/file.pdf", "application/pdf", []byte("%PDF"))
		s := newTestFeedService(mock)

		content := newTestArticleContent("https://example.com/file.pdf")
		s.enrichFromLinkedPage(context.Background(), content)

		assert.Empty(t, content.HTML)
	})

	t.Run("leaves content empty when readability finds nothing", func(t *testing.T) {
		mock := mocks.NewMockWebFetchClient()
		mock.SetHTML("https://example.com/empty", "<html><body></body></html>")
		s := newTestFeedService(mock)

		content := newTestArticleContent("https://example.com/empty")
		s.enrichFromLinkedPage(context.Background(), content)

		assert.Empty(t, content.HTML)
	})
}

func TestBackfillContentHTML_NoSourceURL(t *testing.T) {
	s := newTestFeedService(mocks.NewMockWebFetchClient())

	t.Run("nil source URL", func(t *testing.T) {
		//nolint:exhaustruct // only SourceURL/ID matter here
		book := &models.Book{ID: uuid.New()}
		assert.Empty(t, s.BackfillContentHTML(context.Background(), book))
	})

	t.Run("empty source URL", func(t *testing.T) {
		empty := ""
		//nolint:exhaustruct // only SourceURL/ID matter here
		book := &models.Book{ID: uuid.New(), SourceURL: &empty}
		assert.Empty(t, s.BackfillContentHTML(context.Background(), book))
	})
}

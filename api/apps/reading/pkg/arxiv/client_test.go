package arxiv_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
)

func TestParseID(t *testing.T) {
	tests := []struct {
		in     string
		wantID string
		wantOK bool
	}{
		{"https://arxiv.org/abs/2401.12345", "2401.12345", true},
		{"https://arxiv.org/abs/2401.12345v2", "2401.12345", true},
		{"https://www.arxiv.org/abs/2401.12345", "2401.12345", true},
		{"http://export.arxiv.org/abs/2401.12345", "2401.12345", true},
		{"https://arxiv.org/pdf/2401.12345", "2401.12345", true},
		{"https://arxiv.org/pdf/2401.12345v3.pdf", "2401.12345", true},
		{"https://arxiv.org/abs/math.GT/0309136", "math.GT/0309136", true},
		{
			"https://arxiv.org/pdf/cond-mat.str-el/0309136v1.pdf",
			"cond-mat.str-el/0309136",
			true,
		},
		{"https://doi.org/10.48550/arXiv.2401.12345", "2401.12345", true},
		{"2401.12345", "2401.12345", true},
		{"2401.12345v1", "2401.12345", true},
		{"https://arxiv.org/list/cs.AI/recent", "", false},
		{"https://example.com/abs/2401.12345", "", false},
		{"https://theverge.com/some-article", "", false},
		{"not an id", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			id, ok := arxiv.ParseID(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

const atomFixture = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2401.12345v2</id>
    <published>2024-01-22T18:59:59Z</published>
    <title>Attention Is Not
  All You Need</title>
    <summary>  We revisit the
  transformer architecture.
</summary>
    <author><name>Ada Lovelace</name></author>
    <author><name>Alan Turing</name></author>
  </entry>
</feed>`

const atomErrorFixture = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/api/errors#incorrect_id_format</id>
    <title>Error</title>
    <summary>incorrect id format</summary>
  </entry>
</feed>`

func TestGetByID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2401.12345", r.URL.Query().Get("id_list"))
			_, _ = w.Write([]byte(atomFixture))
		},
	))
	t.Cleanup(ts.Close)

	c := arxiv.NewWithBaseURL(logging.NewNopLogger(), ts.URL)
	paper, err := c.GetByID(context.Background(), "2401.12345")
	require.NoError(t, err)

	assert.Equal(t, "2401.12345", paper.ID)
	assert.Equal(t, "Attention Is Not All You Need", paper.Title)
	assert.Equal(t, "We revisit the transformer architecture.", paper.Abstract)
	assert.Equal(t, []string{"Ada Lovelace", "Alan Turing"}, paper.Authors)
	assert.Equal(t, "https://arxiv.org/abs/2401.12345", paper.AbsURL)
	assert.Equal(t, "https://arxiv.org/pdf/2401.12345", paper.PDFURL)
	assert.Equal(t, 2024, paper.Published.Year())
}

func TestGetByID_NotFound(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty feed", `<feed xmlns="http://www.w3.org/2005/Atom"></feed>`},
		{"error entry", atomErrorFixture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte(tt.body))
				},
			))
			t.Cleanup(ts.Close)

			c := arxiv.NewWithBaseURL(logging.NewNopLogger(), ts.URL)
			_, err := c.GetByID(context.Background(), "9999.99999")
			assert.ErrorIs(t, err, arxiv.ErrNotFound)
		})
	}
}

func TestGetByID_UpstreamError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	))
	t.Cleanup(ts.Close)

	c := arxiv.NewWithBaseURL(logging.NewNopLogger(), ts.URL)
	_, err := c.GetByID(context.Background(), "2401.12345")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, arxiv.ErrNotFound)
}

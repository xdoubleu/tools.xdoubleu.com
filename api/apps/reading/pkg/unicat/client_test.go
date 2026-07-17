package unicat_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/pkg/unicat"
)

// Real UniCat MARCXML response for ISBN 9789463107389 (condensed to essential fields).
const fixtureISBN9789463107389 = `<?xml version='1.0' encoding='UTF-8'?>
<srw:searchRetrieveResponse
    xmlns:srw="http://www.loc.gov/zing/srw/"
    xmlns:tel="http://krait.kb.nl/coop/tel/handbook/telterms.html">
  <srw:version>1.1</srw:version>
  <srw:numberOfRecords>1</srw:numberOfRecords>
  <srw:records>
    <srw:record>
      <srw:recordSchema>info:srw/schema/1/marcxml-v1.1</srw:recordSchema>
      <srw:recordPacking>xml</srw:recordPacking>
      <srw:recordData>
        <record xmlns="http://www.loc.gov/MARC21/slim">
          <leader>00000nam  22      a 4500</leader>
          <datafield tag="020" ind1=" " ind2=" ">
            <subfield code="a">9789463107389</subfield>
          </datafield>
          <datafield tag="100" ind1="1" ind2=" ">
            <subfield code="a">Vandenbroucke, Frank,</subfield>
          </datafield>
          <datafield tag="245" ind1="1" ind2="4">
            <subfield code="a">10 franke vragen aan Frank /</subfield>
          </datafield>
          <datafield tag="300" ind1=" " ind2=" ">
            <subfield code="a">127 pagina&#39;s</subfield>
          </datafield>
          <datafield tag="520" ind1="3" ind2=" ">
            <subfield code="a">Frank Vandenbroucke keerde terug.</subfield>
          </datafield>
          <datafield tag="700" ind1="1" ind2=" ">
            <subfield code="a">Coenen, Mark</subfield>
          </datafield>
        </record>
      </srw:recordData>
    </srw:record>
  </srw:records>
</srw:searchRetrieveResponse>`

const fixtureNotFound = `<?xml version='1.0' encoding='UTF-8'?>
<srw:searchRetrieveResponse xmlns:srw="http://www.loc.gov/zing/srw/">
  <srw:version>1.1</srw:version>
  <srw:numberOfRecords>0</srw:numberOfRecords>
</srw:searchRetrieveResponse>`

func TestMain(m *testing.M) {
	unicat.SetBackoffBase(time.Millisecond)
	os.Exit(m.Run())
}

func buildServer(handler http.HandlerFunc) func() {
	srv := httptest.NewServer(handler)
	unicat.SetBaseURL(srv.URL)
	return func() {
		srv.Close()
		unicat.SetBaseURL("https://www.unicat.be/sru")
	}
}

func newClient(t *testing.T) unicat.Client {
	t.Helper()
	return unicat.New(logging.NewNopLogger())
}

func TestGetByISBN_Found(t *testing.T) {
	cleanup := buildServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "isbn=9789463107389", r.URL.Query().Get("query"))
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(fixtureISBN9789463107389))
	})
	defer cleanup()

	c := newClient(t)
	book, err := c.GetByISBN(context.Background(), "9789463107389")
	require.NoError(t, err)
	require.NotNil(t, book)

	assert.Equal(t, "10 franke vragen aan Frank", book.Title)
	assert.Equal(t, []string{"Frank Vandenbroucke", "Mark Coenen"}, book.Authors)
	require.NotNil(t, book.ISBN13)
	assert.Equal(t, "9789463107389", *book.ISBN13)
	require.NotNil(t, book.PageCount)
	assert.Equal(t, 127, *book.PageCount)
	require.NotNil(t, book.Description)
	assert.Contains(t, *book.Description, "Frank Vandenbroucke")
}

func TestGetByISBN_NotFound(t *testing.T) {
	cleanup := buildServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(fixtureNotFound))
	})
	defer cleanup()

	c := newClient(t)
	book, err := c.GetByISBN(context.Background(), "9781234567890")
	assert.ErrorIs(t, err, unicat.ErrNotFound)
	assert.Nil(t, book)
}

func TestGetByISBN_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	defer cleanup()

	c := newClient(t)
	_, err := c.GetByISBN(context.Background(), "9789463107389")
	assert.Error(t, err)
	assert.Equal(t, 4, attempts, "should retry 4 times on 503")
}

func TestGetByISBN_ClientError_NoRetry(t *testing.T) {
	attempts := 0
	cleanup := buildServer(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	})
	defer cleanup()

	c := newClient(t)
	_, err := c.GetByISBN(context.Background(), "9789463107389")
	assert.Error(t, err)
	assert.Equal(t, 1, attempts, "4xx should not be retried")
}

func TestSearch_Found(t *testing.T) {
	cleanup := buildServer(func(w http.ResponseWriter, r *http.Request) {
		cql := r.URL.Query().Get("query")
		assert.Contains(t, cql, "title=")
		assert.Contains(t, cql, "author=")
		assert.NotContains(t, cql, "dc.")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(fixtureISBN9789463107389))
	})
	defer cleanup()

	c := newClient(t)
	books, err := c.Search(
		context.Background(),
		`intitle:"10 franke vragen aan Frank" inauthor:"Vandenbroucke"`,
	)
	require.NoError(t, err)
	require.Len(t, books, 1)
	assert.Equal(t, "10 franke vragen aan Frank", books[0].Title)
}

func TestSearch_Found_TitleOnly(t *testing.T) {
	cleanup := buildServer(func(w http.ResponseWriter, r *http.Request) {
		cql := r.URL.Query().Get("query")
		assert.Contains(t, cql, "title=")
		assert.NotContains(t, cql, "author=")
		assert.NotContains(t, cql, "dc.")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(fixtureISBN9789463107389))
	})
	defer cleanup()

	c := newClient(t)
	books, err := c.Search(
		context.Background(), `intitle:"10 franke vragen aan Frank"`,
	)
	require.NoError(t, err)
	require.Len(t, books, 1)
}

func TestSearch_EmptyTitle_ReturnsNil(t *testing.T) {
	// Should not hit the server at all when query has no extractable title.
	called := false
	cleanup := buildServer(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	c := newClient(t)
	books, err := c.Search(context.Background(), "inauthor:\"Tolkien\"")
	require.NoError(t, err)
	assert.Nil(t, books)
	assert.False(t, called, "should not call server when title is empty")
}

func TestGetByISBN_MultipleAuthors(t *testing.T) {
	fixture := `<?xml version='1.0' encoding='UTF-8'?>
<srw:searchRetrieveResponse xmlns:srw="http://www.loc.gov/zing/srw/">
  <srw:version>1.1</srw:version>
  <srw:numberOfRecords>1</srw:numberOfRecords>
  <srw:records>
    <srw:record>
      <srw:recordData>
        <record xmlns="http://www.loc.gov/MARC21/slim">
          <datafield tag="100" ind1="1" ind2=" ">
            <subfield code="a">Author, First,</subfield>
          </datafield>
          <datafield tag="700" ind1="1" ind2=" ">
            <subfield code="a">Author, Second</subfield>
          </datafield>
          <datafield tag="700" ind1="1" ind2=" ">
            <subfield code="a">Author, Third,</subfield>
          </datafield>
          <datafield tag="700" ind1="1" ind2=" ">
            <subfield code="a">King, Martin Luther, Jr.</subfield>
          </datafield>
          <datafield tag="245" ind1="1" ind2="0">
            <subfield code="a">Test Book</subfield>
          </datafield>
        </record>
      </srw:recordData>
    </srw:record>
  </srw:records>
</srw:searchRetrieveResponse>`

	cleanup := buildServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(fixture))
	})
	defer cleanup()

	c := newClient(t)
	book, err := c.GetByISBN(context.Background(), "9780000000000")
	require.NoError(t, err)
	require.NotNil(t, book)
	// Trailing commas/periods stripped, and MARC "Last, First" personal names
	// flipped to "First Last". Names with more than one comma (suffixes like
	// "Jr.") pass through unchanged rather than being flipped wrong.
	assert.Equal(
		t,
		[]string{
			"First Author", "Second Author", "Third Author",
			"King, Martin Luther, Jr",
		},
		book.Authors,
	)
}

func TestGetByISBN_MalformedXML(t *testing.T) {
	cleanup := buildServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte("<broken><xml>"))
	})
	defer cleanup()

	c := newClient(t)
	_, err := c.GetByISBN(context.Background(), "9789463107389")
	assert.Error(t, err)
}

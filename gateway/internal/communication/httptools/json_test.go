package httptools_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/communication/httptools"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	err := httptools.WriteJSON(
		rec,
		http.StatusOK,
		map[string]string{"release": "abc"},
		http.Header{"X-Test": []string{"1"}},
	)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("content-type"))
	assert.Equal(t, "1", rec.Header().Get("X-Test"))
	assert.Contains(t, rec.Body.String(), `"release": "abc"`)
}

type brokenWriter struct {
	http.ResponseWriter
}

func (brokenWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteJSONWriteError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	err := httptools.WriteJSON(brokenWriter{rec}, http.StatusOK, "data", nil)
	require.Error(t, err)
}

func TestWriteJSONMarshalError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	err := httptools.WriteJSON(rec, http.StatusOK, make(chan int), nil)
	require.Error(t, err)
}

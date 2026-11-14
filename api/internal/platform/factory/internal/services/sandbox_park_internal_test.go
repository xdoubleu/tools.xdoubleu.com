package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestDigestReadsTheSecondField(t *testing.T) {
	body, status := "abc123 deadbeef\n", http.StatusOK
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/downloads/"+JobImageManifest, r.URL.Path)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	src := ManifestDigest(srv.URL+"/", srv.Client())

	got, err := src(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "deadbeef", got)

	body = "garbage"
	_, err = src(t.Context())
	require.Error(t, err)

	body, status = "", http.StatusNotFound
	_, err = src(t.Context())
	require.Error(t, err)
}

package auth

import (
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rangeService(url string) *LocalService {
	//nolint:exhaustruct //only the range URL matters
	return &LocalService{PwnedRangeURL: url}
}

func pwnedServer(t *testing.T, password string, count string) *httptest.Server {
	t.Helper()
	sum := sha1.Sum([]byte(password))
	digest := strings.ToUpper(hex.EncodeToString(sum[:]))

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/range/"+digest[:5], r.URL.Path)
		assert.Equal(t, "true", r.Header.Get("Add-Padding"))
		_, _ = w.Write([]byte("0018A45C4D1DEF81644B54AB7F969B88D65:3\r\n" +
			digest[5:] + ":" + count + "\r\n"))
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckPasswordLength(t *testing.T) {
	require.ErrorIs(t, checkPasswordLength("short"), ErrPasswordTooShort)
	require.NoError(t, checkPasswordLength(strings.Repeat("a", 72)))
	require.ErrorIs(t, checkPasswordLength(strings.Repeat("a", 73)), ErrPasswordTooLong)
	// Bytes, not characters: 37 two-byte runes are 74 bytes.
	require.ErrorIs(t, checkPasswordLength(strings.Repeat("é", 37)), ErrPasswordTooLong)
}

func TestCheckPasswordBreached_Breached(t *testing.T) {
	srv := pwnedServer(t, "password123", "9001")
	service := rangeService(srv.URL + "/range/")

	err := service.checkPasswordBreached(t.Context(), "password123")
	require.ErrorIs(t, err, ErrPasswordBreached)
	assert.True(t, IsPasswordPolicyError(err))
}

func TestCheckPasswordBreached_PaddingIgnored(t *testing.T) {
	srv := pwnedServer(t, "a-fresh-passphrase", "0")
	service := rangeService(srv.URL + "/range/")

	require.NoError(t, service.checkPasswordBreached(t.Context(), "a-fresh-passphrase"))
}

func TestCheckPasswordBreached_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL + "/range/"
	srv.Close()
	service := rangeService(url)

	require.NoError(t, service.checkPasswordBreached(t.Context(), "password123"))
}

func TestCheckPasswordBreached_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	service := rangeService(srv.URL + "/range/")

	require.NoError(t, service.checkPasswordBreached(t.Context(), "password123"))
}

func TestCheckPasswordBreached_Slow(t *testing.T) {
	release := make(chan struct{})
	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	service := rangeService(srv.URL + "/range/")

	start := time.Now()
	require.NoError(t, service.checkPasswordBreached(t.Context(), "password123"))
	assert.Less(t, time.Since(start), 4*time.Second)
}

func TestCheckPasswordBreached_Disabled(t *testing.T) {
	require.NoError(t, rangeService("").checkPasswordBreached(t.Context(), "password123"))
}

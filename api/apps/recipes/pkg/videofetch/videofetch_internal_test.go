package videofetch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYouTubeID(t *testing.T) {
	tests := []struct {
		url string
		id  string
		err error
	}{
		{"https://www.youtube.com/shorts/XsipAaImDVc", "XsipAaImDVc", nil},
		{"https://youtube.com/shorts/XsipAaImDVc?feature=share", "XsipAaImDVc", nil},
		{"https://m.youtube.com/watch?v=XsipAaImDVc&t=3", "XsipAaImDVc", nil},
		{"https://youtu.be/XsipAaImDVc?si=abc", "XsipAaImDVc", nil},
		{"http://www.youtube.com/watch?v=XsipAaImDVc", "XsipAaImDVc", nil},
		{"https://www.instagram.com/reel/CtMXPf7gIB0/", "", ErrInstagram},
		{"https://instagram.com/p/CtMXPf7gIB0", "", ErrInstagram},
		{"https://www.tiktok.com/@x/video/1", "", ErrUnsupportedURL},
		{"https://youtube.com.evil.test/shorts/XsipAaImDVc", "", ErrUnsupportedURL},
		{"ftp://youtu.be/XsipAaImDVc", "", ErrUnsupportedURL},
		{"https://www.youtube.com/shorts/short", "", ErrUnsupportedURL},
		{"https://www.youtube.com/watch", "", ErrUnsupportedURL},
		{"https://www.youtube.com/@channel", "", ErrUnsupportedURL},
		{"://bad", "", ErrUnsupportedURL},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			id, err := youTubeID(tt.url)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.id, id)
		})
	}
}

func fixtureServer(t *testing.T, status int, fixture string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/youtubei/v1/next", r.URL.Path)

			var body struct {
				VideoID string `json:"videoId"`
			}
			raw, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.NoError(t, json.Unmarshal(raw, &body))
			assert.Equal(t, "XsipAaImDVc", body.VideoID)

			w.WriteHeader(status)
			if fixture == "" {
				_, _ = w.Write([]byte("not json"))
				return
			}
			data, err := os.ReadFile("testdata/" + fixture)
			assert.NoError(t, err)
			_, _ = w.Write(data)
		}))
	t.Cleanup(srv.Close)
	return srv
}

func testFetcher(srv *httptest.Server) *Fetcher {
	f := New(true)
	f.baseURL = srv.URL
	return f
}

func TestFetch_YouTubeShort(t *testing.T) {
	srv := fixtureServer(t, http.StatusOK, "youtube-next-short.json")

	video, err := testFetcher(srv).Fetch(
		context.Background(), "https://youtube.com/shorts/XsipAaImDVc",
	)
	require.NoError(t, err)
	assert.Equal(t, "youtube", video.Platform)
	assert.Equal(t, "https://www.youtube.com/shorts/XsipAaImDVc", video.URL)
	assert.Equal(t,
		"White Sauce Pasta | Creamy & Cheesy White Sauce Pasta | Kanak's Kitchen",
		video.Title)
	assert.Contains(t, video.Description, "INGREDIENTS:\nBOILING:\n2 cups Penne")
}

func TestFetch_Errors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		fixture string
		err     error
	}{
		{"unavailable video", http.StatusOK, "youtube-next-missing.json", ErrNotFound},
		{"http error", http.StatusTooManyRequests, "youtube-next-short.json", ErrFetch},
		{"invalid json", http.StatusOK, "", ErrFetch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := fixtureServer(t, tt.status, tt.fixture)
			_, err := testFetcher(srv).Fetch(
				context.Background(), "https://youtu.be/XsipAaImDVc",
			)
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestFetch_UnsupportedURLMakesNoRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("unexpected request")
		}))
	t.Cleanup(srv.Close)

	_, err := testFetcher(srv).Fetch(
		context.Background(), "https://www.instagram.com/reel/CtMXPf7gIB0/",
	)
	require.ErrorIs(t, err, ErrInstagram)
}

func TestFetch_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	f := testFetcher(srv)
	srv.Close()

	_, err := f.Fetch(context.Background(), "https://youtu.be/XsipAaImDVc")
	require.ErrorIs(t, err, ErrFetch)
}

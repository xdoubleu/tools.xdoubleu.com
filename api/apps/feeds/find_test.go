package feeds_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
)

func TestFindSource(t *testing.T) {
	ctx := context.Background()
	const uid = "feeds-find-test-user"
	_, err := testDB.Exec(ctx, "DELETE FROM feeds.feeds WHERE user_id = $1", uid)
	require.NoError(t, err)

	mockWebFetch.SetHTML("http://plain.example/", "<html></html>")
	m, err := testApp.FindSource(ctx, uid, "Plain", "http://plain.example/")
	require.NoError(t, err)
	require.NotNil(t, m, "plain http works too")

	mockWebFetch.SetHTML("https://blog.example/", "<html></html>")
	m, err = testApp.FindSource(ctx, uid, "Example Blog", "https://blog.example/")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "https://blog.example/", m.URL)
	assert.Equal(t, "https://blog.example/", m.Ref)
	assert.Equal(t, "Example Blog", m.Title)
	assert.False(t, m.Known)

	_, err = testDB.Exec(ctx, `INSERT INTO feeds.feeds (user_id, url, title)
		VALUES ($1, 'https://feeds.blog.example/rss', 'Example')`, uid)
	require.NoError(t, err)
	m, err = testApp.FindSource(ctx, uid, "Example Blog", "https://www.blog.example/")
	require.NoError(t, err)
	assert.Nil(t, m, "www.blog.example isn't mocked, so unreachable")
	mockWebFetch.SetHTML("https://www.blog.example/", "<html></html>")
	m, err = testApp.FindSource(ctx, uid, "Example Blog", "https://www.blog.example/")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.True(t, m.Known, "www. and feeds. subdomains are the same site")

	//nolint:exhaustruct // a redirect only needs its final URL
	mockWebFetch.Responses["https://old.example/"] = &webfetch.Result{
		FinalURL: "https://blog.example/",
	}
	m, err = testApp.FindSource(ctx, uid, "Moved", "https://old.example/")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.True(t, m.Known, "known after the redirect")
	assert.Equal(t, "https://blog.example/", m.URL)

	mockWebFetch.Errs["https://huge.example/"] = webfetch.ErrTooLarge
	m, err = testApp.FindSource(ctx, uid, "Huge", "https://huge.example/")
	require.NoError(t, err)
	require.NotNil(t, m, "a page over the cap still exists")
	assert.Equal(t, "https://huge.example/", m.URL)
	assert.False(t, m.Known)

	for _, bad := range []string{
		"ftp://blog.example/", "not a url", "https://unreachable.example/",
	} {
		m, err = testApp.FindSource(ctx, uid, "X", bad)
		require.NoError(t, err)
		assert.Nil(t, m, bad)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = testApp.FindSource(cancelled, uid, "X", "https://blog.example/")
	require.Error(t, err)
}

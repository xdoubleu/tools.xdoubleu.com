package feeds_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindSource(t *testing.T) {
	ctx := context.Background()
	const uid = "feeds-find-test-user"
	_, err := testDB.Exec(ctx, "DELETE FROM feeds.feeds WHERE user_id = $1", uid)
	require.NoError(t, err)

	mockWebFetch.SetHTML("https://blog.example/", "<html></html>")
	m, err := testApp.FindSource(ctx, uid, "Example Blog", "https://blog.example/")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "https://blog.example/", m.URL)
	assert.Equal(t, "https://blog.example/", m.Ref)
	assert.Equal(t, "Example Blog", m.Title)
	assert.False(t, m.Known)

	_, err = testDB.Exec(ctx, `INSERT INTO feeds.feeds (user_id, url, title)
		VALUES ($1, 'https://www.blog.example/feed.xml', 'Example')`, uid)
	require.NoError(t, err)
	m, err = testApp.FindSource(ctx, uid, "Example Blog", "https://blog.example/")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.True(t, m.Known, "same host, www. ignored")

	for _, bad := range []string{"ftp://blog.example/", "not a url", "https://unreachable.example/"} {
		m, err = testApp.FindSource(ctx, uid, "X", bad)
		require.NoError(t, err)
		assert.Nil(t, m, bad)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = testApp.FindSource(cancelled, uid, "X", "https://blog.example/")
	require.Error(t, err)
}

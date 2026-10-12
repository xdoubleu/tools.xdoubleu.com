package podcasts_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindShow(t *testing.T) {
	const uid = "podcasts-find-test-user"
	c := newClient(t, uid, fakeITunes{})
	a := newApp(uid, fakeITunes{}, nil)
	ctx := context.Background()

	m, err := a.FindShow(ctx, uid, "Hardcore History")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "1", m.Ref)
	assert.Equal(t, "Dan Carlin", m.Creator)
	assert.Equal(t, "https://img.example/1.jpg", m.ImageURL)
	assert.Equal(t, "https://podcasts.apple.com/podcast/id1", m.URL)
	assert.False(t, m.Known)

	add(t, c, 1)
	m, err = a.FindShow(ctx, uid, "hardcore history")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.True(t, m.Known)

	m, err = a.FindShow(ctx, uid, "History")
	require.NoError(t, err)
	assert.Nil(t, m, "a partial title is no match")

	_, err = newApp(uid, downITunes{}, nil).FindShow(ctx, uid, "Hardcore History")
	assert.Error(t, err)
}

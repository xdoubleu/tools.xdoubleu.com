package services

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/app"
)

func status(t *testing.T, err error) int {
	t.Helper()
	var httpErr *app.HTTPError
	require.True(t, errors.As(err, &httpErr))
	return httpErr.Status
}

func TestNormaliseTitleBounds(t *testing.T) {
	_, err := normalise(TaskInput{Title: strings.Repeat("a", maxTitleLen)})
	assert.NoError(t, err)
	_, err = normalise(TaskInput{Title: strings.Repeat("a", maxTitleLen+1)})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
	_, err = normalise(TaskInput{Title: " \t"})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
}

func TestNormaliseBodyBounds(t *testing.T) {
	_, err := normalise(TaskInput{Title: "x", Body: strings.Repeat("a", maxBodyLen)})
	assert.NoError(t, err)
	_, err = normalise(TaskInput{Title: "x", Body: strings.Repeat("a", maxBodyLen+1)})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
}

func TestNormalisePriority(t *testing.T) {
	got, err := normalise(TaskInput{Title: "x"})
	require.NoError(t, err)
	assert.Equal(t, models.PriorityP2, got.Priority)

	got, err = normalise(TaskInput{Title: "x", Priority: models.PriorityP0})
	require.NoError(t, err)
	assert.Equal(t, models.PriorityP0, got.Priority)

	_, err = normalise(TaskInput{Title: "x", Priority: models.PriorityInvalid})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
}

func TestNormaliseLabels(t *testing.T) {
	got, err := normalise(TaskInput{Title: "x", Labels: []string{" a ", "a", "", "b"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, got.Labels)

	got, err = normalise(TaskInput{Title: "x"})
	require.NoError(t, err)
	assert.Equal(t, []string{}, got.Labels, "never nil, so the column stays an array")

	_, err = normalise(
		TaskInput{Title: "x", Labels: []string{strings.Repeat("a", maxLabelLen)}},
	)
	assert.NoError(t, err)
	_, err = normalise(TaskInput{
		Title: "x", Labels: []string{strings.Repeat("a", maxLabelLen+1)},
	})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
}

func TestNormaliseLabelCountBounds(t *testing.T) {
	labels := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = strings.Repeat("l", i+1)
		}
		return out
	}
	_, err := normalise(TaskInput{Title: "x", Labels: labels(maxLabels)})
	assert.NoError(t, err)
	_, err = normalise(TaskInput{Title: "x", Labels: labels(maxLabels + 1)})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
}

func TestNormaliseComment(t *testing.T) {
	got, err := normaliseComment("  hi  ")
	require.NoError(t, err)
	assert.Equal(t, "hi", got)

	_, err = normaliseComment("   ")
	assert.Equal(t, http.StatusBadRequest, status(t, err))

	_, err = normaliseComment(strings.Repeat("a", maxCommentLen))
	assert.NoError(t, err)
	_, err = normaliseComment(strings.Repeat("a", maxCommentLen+1))
	assert.Equal(t, http.StatusBadRequest, status(t, err))
}

func TestCheckMove(t *testing.T) {
	human := models.Actor{Kind: models.ActorHuman, ID: "u", Browser: true, Admin: true}
	mcp := models.Actor{Kind: models.ActorHuman, ID: "u", Browser: false, Admin: true}

	assert.NoError(t, checkMove(human, models.StatusReady))
	assert.NoError(t, checkMove(mcp, models.StatusInProgress))
	assert.Equal(t, http.StatusForbidden, status(t, checkMove(mcp, models.StatusReady)))
	assert.Equal(t, http.StatusBadRequest, status(t, checkMove(human, "")))
}

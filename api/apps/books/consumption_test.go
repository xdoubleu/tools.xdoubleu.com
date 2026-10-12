package books_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/internal/consumption"
)

const consumptionUserID = "books-consumption-test-user"

// addConsumptionBook adds a book to consumptionUserID's library and sets its
// row directly.
func addConsumptionBook(
	t *testing.T,
	title, status string,
	rating *int16,
	tags []string,
	finished []time.Time,
) *models.UserBook {
	t.Helper()
	ctx := context.Background()
	//nolint:exhaustruct // ISBN intentionally absent
	ub, err := testApp.Services.Books.AddToLibrary(ctx, consumptionUserID,
		services.SourceProposal{
			Source: "manual", Title: title, Authors: []string{"A", "B", "C"},
		},
		models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, `
		UPDATE books.user_books
		SET status = $2, rating = $3, tags = COALESCE($4, ARRAY[]::TEXT[]),
		    finished_at = COALESCE($5, ARRAY[]::TIMESTAMPTZ[])
		WHERE id = $1`, ub.ID, status, rating, tags, finished)
	require.NoError(t, err)
	return ub
}

func TestListConsumption(t *testing.T) {
	ctx := context.Background()
	_, err := testDB.Exec(ctx,
		"DELETE FROM books.user_books WHERE user_id = $1", consumptionUserID)
	require.NoError(t, err)

	five := int16(5)
	finished := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	rated := addConsumptionBook(t, "Consumption Rated", models.StatusRead, &five,
		[]string{models.TagFavourite, "sci-fi"},
		[]time.Time{finished.AddDate(-1, 0, 0), finished})
	addConsumptionBook(t, "Consumption Dropped", models.StatusDropped, nil, nil, nil)
	addConsumptionBook(t, "Consumption Wishlist", models.StatusToRead, nil, nil, nil)

	events, err := testApp.ListConsumption(ctx, consumptionUserID)
	require.NoError(t, err)
	require.Len(t, events, 2)

	byTitle := map[string]consumption.Event{}
	for _, e := range events {
		byTitle[e.Title] = e
	}
	got := byTitle["Consumption Rated"]
	assert.Equal(t, consumption.DomainBooks, got.Domain)
	assert.Equal(t, rated.BookID.String(), got.ItemID)
	assert.Equal(t, "A, B", got.Creator)
	assert.Equal(t, []string{"sci-fi"}, got.Tags)
	assert.True(t, finished.Equal(got.At))
	assert.Equal(t, consumption.StrongPositive, got.Signal)
	assert.Equal(t, "rated 5★", got.Reason)

	dropped := byTitle["Consumption Dropped"]
	assert.Equal(t, consumption.Negative, dropped.Signal)
	assert.Equal(t, "abandoned", dropped.Reason)
	assert.True(t, dropped.At.IsZero(), "a book never finished stays undated")
}

func TestListConsumption_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := testApp.ListConsumption(ctx, "consumption-error-user")
	assert.Error(t, err)
}

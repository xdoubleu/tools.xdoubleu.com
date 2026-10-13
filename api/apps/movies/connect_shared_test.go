package movies_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
	"tools.xdoubleu.com/internal/consumption"
)

// joinFamily puts the users in one new family and returns its id.
func joinFamily(t *testing.T, users ...string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var familyID uuid.UUID
	require.NoError(t, testDB.QueryRow(ctx,
		"INSERT INTO global.families DEFAULT VALUES RETURNING id").Scan(&familyID))
	for _, u := range users {
		_, err := testDB.Exec(ctx, "DELETE FROM global.family_members WHERE user_id = $1", u)
		require.NoError(t, err)
		_, err = testDB.Exec(ctx,
			"INSERT INTO global.family_members (user_id, family_id) VALUES ($1, $2)",
			u, familyID)
		require.NoError(t, err)
	}
	_, err := testDB.Exec(ctx,
		"DELETE FROM movies.user_titles WHERE user_id = $1",
		consumption.SharedSubject(familyID))
	require.NoError(t, err)
	return familyID
}

func TestSharedList_SeparateFromSoloAndSeenByFamily(t *testing.T) {
	const outsider = "movies-test-outsider"
	joinFamily(t, userID, otherUserID)
	joinFamily(t, outsider)
	mine := newClient(t, userID, fakeTMDB{})
	partner := newClient(t, otherUserID, fakeTMDB{})
	stranger := newClient(t, outsider, fakeTMDB{})
	ctx := context.Background()

	solo := add(t, mine, "movie", 603, "want")
	shared, err := mine.AddTitle(ctx, &moviesv1.AddTitleRequest{
		MediaType: "movie", TmdbId: 603, Status: "watched", Shared: true,
	})
	require.NoError(t, err)
	assert.NotEqual(t, solo.Id, shared.Entry.Id, "a title sits on both lists")
	assert.Equal(t, "want", getTitle(t, mine, solo.Id).Entry.Status)

	list := func(
		c moviesv1connect.MoviesServiceClient,
		sharedList bool,
	) []*moviesv1.BacklogEntry {
		resp, listErr := c.ListBacklog(ctx, &moviesv1.ListBacklogRequest{Shared: sharedList})
		require.NoError(t, listErr)
		return resp.Entries
	}
	require.Len(t, list(partner, true), 1, "the family sees the shared list")
	assert.Equal(t, shared.Entry.Id, list(partner, true)[0].Id)
	assert.Empty(t, list(partner, false), "the solo list stays private")
	assert.Empty(t, list(stranger, true), "other families see nothing")
	assert.Len(t, list(mine, false), 1)

	// The partner edits the shared entry; the solo one is out of reach.
	rated, err := partner.SetRating(ctx, &moviesv1.SetRatingRequest{
		Id: shared.Entry.Id, Rating: stars(5), Shared: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(5), rated.Entry.GetRating())
	_, err = partner.SetRating(ctx, &moviesv1.SetRatingRequest{
		Id: solo.Id, Rating: stars(1),
	})
	assert.Error(t, err)
	_, err = stranger.SetRating(ctx, &moviesv1.SetRatingRequest{
		Id: shared.Entry.Id, Rating: stars(1), Shared: true,
	})
	assert.Error(t, err)

	stats, err := partner.GetStats(ctx, &moviesv1.GetStatsRequest{Shared: true})
	require.NoError(t, err)
	assert.Equal(t, int32(1), stats.MoviesWatched)
	soloStats, err := mine.GetStats(ctx, &moviesv1.GetStatsRequest{})
	require.NoError(t, err)
	assert.Equal(t, int32(0), soloStats.MoviesWatched)

	_, err = partner.RemoveTitle(ctx, &moviesv1.RemoveTitleRequest{
		Id: shared.Entry.Id, Shared: true,
	})
	require.NoError(t, err)
	assert.Empty(t, list(mine, true))
}

func TestSharedList_ConsumptionForTaste(t *testing.T) {
	familyID := joinFamily(t, userID)
	c := newClient(t, userID, fakeTMDB{})
	_, err := c.AddTitle(context.Background(), &moviesv1.AddTitleRequest{
		MediaType: "movie", TmdbId: 603, Status: "watched", Shared: true,
	})
	require.NoError(t, err)
	subject := consumption.SharedSubject(familyID)
	a := newApp(userID, fakeTMDB{})

	subjects, err := a.ListSharedSubjects(context.Background())
	require.NoError(t, err)
	assert.Contains(t, subjects, subject)

	events, err := a.ListConsumption(context.Background(), subject)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "The Matrix", events[0].Title)
	solo, err := a.ListConsumption(context.Background(), userID)
	require.NoError(t, err)
	assert.Empty(t, solo, "shared watches never feed the solo profile")
}

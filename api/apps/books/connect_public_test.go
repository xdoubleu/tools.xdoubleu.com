package books_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/gen/books/v1/booksv1connect"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

// publicUserID owns the data behind the public-profile tests. It is distinct
// from userID so parallel test packages sharing the DB (cmd/api's
// ProfileService tests use userID) never fight over the same
// global.profile_shares row.
const publicUserID = "dddddddd-1111-2222-3333-444444444444"

const publicBooksToken = "test-books-profile-token"

// ensureProfileShare mirrors cmd/api/migrations/00006_profile_shares.sql so
// these tests can run before the cmd/api package has applied the global
// migrations, then links publicBooksToken to publicUserID.
func ensureProfileShare(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	stmts := []string{
		"CREATE SCHEMA IF NOT EXISTS global",
		`CREATE TABLE IF NOT EXISTS global.profile_shares (
			user_id TEXT PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	}
	for _, stmt := range stmts {
		_, err := testDB.Exec(ctx, stmt)
		require.NoError(t, err)
	}

	repo := sharedrepos.NewProfileSharesRepository(testDB)
	_, err := repo.Upsert(ctx, publicUserID, publicBooksToken)
	require.NoError(t, err)
}

// addPublicUserBook adds a favourite-tagged wishlist book for publicUserID.
func addPublicUserBook(t *testing.T, title string) *models.UserBook {
	t.Helper()
	ext := services.SourceProposal{ //nolint:exhaustruct //Index/Differs unused
		Source:  "manual",
		Title:   title,
		Authors: []string{"Public Author"},
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(),
		publicUserID,
		ext,
		models.StatusToRead,
		[]string{models.TagFavourite},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// newPublicBooksClient returns a client with NO auth cookie — the public
// service must work without a session.
func newPublicBooksClient(t *testing.T) booksv1connect.PublicLibraryServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return booksv1connect.NewPublicLibraryServiceClient(http.DefaultClient, ts.URL)
}

func TestGetSharedLibrary_Success(t *testing.T) {
	ensureProfileShare(t)
	book := addPublicUserBook(t, "Public Profile Book")

	// Remove any Kobo devices left over from other tests (DB state persists
	// across runs) so the empty last-synced assertion is deterministic.
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM books.kobo_devices WHERE user_id = $1", publicUserID)
	require.NoError(t, err)

	client := newPublicBooksClient(t)
	resp, err := client.GetSharedLibrary(
		context.Background(),
		connect.NewRequest(&booksv1.GetSharedLibraryRequest{
			Token: publicBooksToken,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Library)
	require.NotEmpty(t, resp.Msg.Library.Wishlist)
	assert.Empty(t, resp.Msg.LastSyncedAt,
		"user without Kobo devices has no last synced timestamp")

	var found *booksv1.UserBook
	for _, ub := range resp.Msg.Library.Wishlist {
		if ub.BookId == book.BookID.String() {
			found = ub
			break
		}
	}
	require.NotNil(t, found, "added book should be in the shared wishlist")
	assert.Contains(t, found.Tags, models.TagFavourite,
		"tags (including favourite) must be exposed on the shared library")
}

func TestGetSharedLibrary_LastSyncedAt(t *testing.T) {
	ensureProfileShare(t)

	_, err := testDB.Exec(context.Background(), `
		INSERT INTO books.kobo_devices (user_id, name, token_hash, last_seen_at)
		VALUES ($1, 'Test Kobo', 'public-profile-test-hash', now())
		ON CONFLICT (token_hash) DO UPDATE SET last_seen_at = now()
	`, publicUserID)
	require.NoError(t, err)

	client := newPublicBooksClient(t)
	resp, err := client.GetSharedLibrary(
		context.Background(),
		connect.NewRequest(&booksv1.GetSharedLibraryRequest{
			Token: publicBooksToken,
		}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.LastSyncedAt,
		"most recent Kobo sync should be exposed")
}

func TestGetSharedLibrary_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicBooksClient(t)
	_, err := client.GetSharedLibrary(
		context.Background(),
		connect.NewRequest(&booksv1.GetSharedLibraryRequest{
			Token: "definitely-not-a-token",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedLibrary_EmptyToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicBooksClient(t)
	_, err := client.GetSharedLibrary(
		context.Background(),
		connect.NewRequest(&booksv1.GetSharedLibraryRequest{}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedBooksProgress_Success(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicBooksClient(t)
	resp, err := client.GetSharedBooksProgress(
		context.Background(),
		connect.NewRequest(&booksv1.GetSharedBooksProgressRequest{
			Token: publicBooksToken,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Progress)
	assert.NotEmpty(t, resp.Msg.Progress.DateStart)
	assert.NotEmpty(t, resp.Msg.Progress.DateEnd)
}

func TestGetSharedBooksProgress_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicBooksClient(t)
	_, err := client.GetSharedBooksProgress(
		context.Background(),
		connect.NewRequest(&booksv1.GetSharedBooksProgressRequest{
			Token: "definitely-not-a-token",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/database/postgres"
)

// fakeDB is a minimal postgres.DB double that just records calls.
type fakeDB struct {
	execCalled     bool
	queryCalled    bool
	queryRowCalled bool
	sendBatch      bool
	begin          bool
	beginTx        bool
	ping           bool
}

func (f *fakeDB) Exec(
	_ context.Context, _ string, _ ...any,
) (pgconn.CommandTag, error) {
	f.execCalled = true
	return pgconn.CommandTag{}, nil
}

func (f *fakeDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	f.queryCalled = true
	return nil, nil //nolint:nilnil //test double
}

func (f *fakeDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	f.queryRowCalled = true
	return nil
}

func (f *fakeDB) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	f.sendBatch = true
	return nil
}

func (f *fakeDB) Begin(_ context.Context) (pgx.Tx, error) {
	f.begin = true
	return nil, nil //nolint:nilnil //test double
}

func (f *fakeDB) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	f.beginTx = true
	return nil, nil //nolint:nilnil //test double
}

func (f *fakeDB) Ping(_ context.Context) error {
	f.ping = true
	return nil
}

func TestSpanDBWrapsEveryCall(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct //call-tracking flags are zero-value, set by the calls under test
	fake := &fakeDB{}
	db := postgres.NewSpanDB(fake)
	ctx := context.Background()

	_, err := db.Exec(ctx, "select 1")
	require.NoError(t, err)
	assert.True(t, fake.execCalled)

	//nolint:sqlclosecheck //rows is always nil from the test double, nothing to close
	rows, err := db.Query(ctx, "select 1")
	require.NoError(t, err)
	assert.Nil(t, rows)
	assert.True(t, fake.queryCalled)

	db.QueryRow(ctx, "select 1")
	assert.True(t, fake.queryRowCalled)

	//nolint:exhaustruct //QueuedQueries is populated internally by pgx.Batch.Queue
	db.SendBatch(ctx, &pgx.Batch{})
	assert.True(t, fake.sendBatch)

	_, err = db.Begin(ctx)
	require.NoError(t, err)
	assert.True(t, fake.begin)

	_, err = db.BeginTx(ctx, pgx.TxOptions{}) //nolint:exhaustruct //zero value is fine
	require.NoError(t, err)
	assert.True(t, fake.beginTx)

	require.NoError(t, db.Ping(ctx))
	assert.True(t, fake.ping)
}

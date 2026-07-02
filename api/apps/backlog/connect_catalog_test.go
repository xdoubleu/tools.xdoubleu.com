package backlog_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
)

func TestConnectImportBooks(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.ImportBooksRequest{
		CsvData: []byte(goodreadsCSVForImport),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ImportBooks(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
	assert.GreaterOrEqual(t, resp.Msg.ImportedCount, int32(0))
}

func TestConnectCompareCSV(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.CompareCSVRequest{
		CsvData: []byte(goodreadsCSVForImport),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.CompareCSV(ctx, req)
	assert.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Msg)
	assert.GreaterOrEqual(t, resp.Msg.CsvCount, int32(0))
	assert.GreaterOrEqual(t, resp.Msg.LibraryCount, int32(0))
}

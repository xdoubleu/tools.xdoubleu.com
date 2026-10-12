package books_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"

	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func TestConnectImportBooks(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.ImportBooksRequest{
		CsvData: []byte(goodreadsCSVForImport),
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.ImportBooks(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.ImportedCount, int32(0))
}

package books_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/testhelper"
)

// tokenHash hashes a raw token like the service (sha256 hex).
func tokenHash(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func TestGetKEPUBStatus_NoFiles(t *testing.T) {
	book := addUniqueBook(t)
	err := testApp.Repositories.Books.UpsertUserBook(
		context.Background(),
		models.UserBook{ //nolint:exhaustruct //optional fields
			UserID: userID,
			BookID: book.ID,
			Status: models.StatusToRead,
			Tags:   []string{},
		},
	)
	require.NoError(t, err)

	result, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.False(t, result.HasEPUB)
	assert.Empty(t, result.KepubStatus)
}

func TestGetKEPUBStatus_EPUBOnly(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)

	result, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.True(t, result.HasEPUB)
	assert.Empty(t, result.KepubStatus)
}

func TestGetKEPUBStatus_PDFOnly(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)

	result, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.False(t, result.HasEPUB)
	assert.True(t, result.HasPDF)
	assert.Empty(t, result.KepubStatus)
}

func TestGetKEPUBStatus_EPUBAndKEPUBReady(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertKEPUBRow(t, bookID, userID)

	result, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.True(t, result.HasEPUB)
	assert.Equal(t, models.FileStatusReady, result.KepubStatus)
	assert.False(t, result.KepubStale, "a current-version KEPUB row must not be stale")
}

// TestGetKEPUBStatus_EPUBAndKEPUBReady_StaleVersion: an older converter
// version is reported as stale.
func TestGetKEPUBStatus_EPUBAndKEPUBReady_StaleVersion(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertStaleKEPUBRow(t, bookID, userID)

	result, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.True(t, result.HasEPUB)
	assert.Equal(t, models.FileStatusReady, result.KepubStatus)
	assert.True(t, result.KepubStale,
		"a KEPUB row stamped with an older converter version must be reported stale")
}

func TestGetKEPUBStatus_EPUBAndKEPUBConverting(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)

	convertingRow := models.BookFile{ //nolint:exhaustruct //optional fields
		BookID:     bookID,
		UserID:     userID,
		Format:     models.FileFormatKEPUB,
		StorageKey: "",
		SizeBytes:  0,
		Status:     models.FileStatusConverting,
	}
	_, err := testApp.Repositories.BookFiles.Insert(context.Background(), convertingRow)
	require.NoError(t, err)

	result, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.True(t, result.HasEPUB)
	assert.Equal(t, models.FileStatusConverting, result.KepubStatus)
}

func TestGetKoboFileFormat_DefaultKEPUB(t *testing.T) {
	ub := addTestBook(t, "KoboFmtDefault-"+uuid.NewString())

	format, err := testApp.Services.Books.GetKoboFileFormat(
		context.Background(), userID, ub.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.FileFormatKEPUB, format)
}

func TestGetKoboFileFormat_PDFTag_ReturnsPDF(t *testing.T) {
	ub := addTestBook(t, "KoboFmtPDF-"+uuid.NewString())
	err := testApp.Repositories.Books.UpdateTags(
		context.Background(), userID, ub.BookID,
		[]string{models.TagKoboFormatPDF},
		false, // no kobo-sync tag
	)
	require.NoError(t, err)

	format, err := testApp.Services.Books.GetKoboFileFormat(
		context.Background(), userID, ub.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.FileFormatPDF, format)
}

func TestEnableKoboSync_SetsTag(t *testing.T) {
	ub := addTestBook(t, "KoboSyncTag-"+uuid.NewString())

	err := testApp.Services.Books.EnableKoboSync(
		context.Background(), userID, ub.BookID,
	)
	require.NoError(t, err)

	updated, err := testApp.Services.Books.GetUserBook(
		context.Background(), userID, ub.BookID,
	)
	require.NoError(t, err)
	assert.True(t, updated.HasTag(models.TagKoboSync))
}

func TestEnableKoboSync_Idempotent(t *testing.T) {
	ub := addTestBook(t, "KoboSyncIdempotent-"+uuid.NewString())

	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), userID, ub.BookID,
	))
	require.NoError(t, testApp.Services.Books.EnableKoboSync(
		context.Background(), userID, ub.BookID,
	))

	updated, err := testApp.Services.Books.GetUserBook(
		context.Background(), userID, ub.BookID,
	)
	require.NoError(t, err)
	count := 0
	for _, tag := range updated.Tags {
		if tag == models.TagKoboSync {
			count++
		}
	}
	assert.Equal(t, 1, count, "kobo-sync tag must appear exactly once")
}

func TestConnectEnableKoboSync_EPUBBook_ReturnsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)

	req := connect.NewRequest(&booksv1.EnableKoboSyncRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.EnableKoboSync(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)

	updated, err := testApp.Services.Books.GetUserBook(ctx, userID, bookID)
	require.NoError(t, err)
	assert.True(t, updated.HasTag(models.TagKoboSync))
}

func TestConnectEnableKoboSync_AlreadyReadyKEPUB_ReturnsReady(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertKEPUBRow(t, bookID, userID)

	req := connect.NewRequest(&booksv1.EnableKoboSyncRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.EnableKoboSync(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusReady, resp.Msg.KepubStatus)
}

// TestConnectEnableKoboSync_StaleReadyKEPUB_ReturnsConverting: a stale KEPUB
// re-triggers conversion.
func TestConnectEnableKoboSync_StaleReadyKEPUB_ReturnsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertStaleKEPUBRow(t, bookID, userID)

	req := connect.NewRequest(&booksv1.EnableKoboSyncRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.EnableKoboSync(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus,
		"a stale ready KEPUB must re-trigger conversion, not be served as-is")
}

func TestConnectEnableKoboSync_PDFOnly_ReturnsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)

	req := connect.NewRequest(&booksv1.EnableKoboSyncRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.EnableKoboSync(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)
}

func TestConnectEnableKoboSync_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.EnableKoboSyncRequest{BookId: "bad-id"})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.EnableKoboSync(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestConnectGetKEPUBStatus_NoFiles(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	book := addUniqueBook(t)
	err := testApp.Repositories.Books.UpsertUserBook(
		context.Background(),
		models.UserBook{ //nolint:exhaustruct //optional fields
			UserID: userID,
			BookID: book.ID,
			Status: models.StatusToRead,
			Tags:   []string{},
		},
	)
	require.NoError(t, err)

	req := connect.NewRequest(
		&booksv1.GetKEPUBStatusRequest{BookId: book.ID.String()},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetKEPUBStatus(ctx, req)
	require.NoError(t, err)
	assert.False(t, resp.Msg.HasEpub)
	assert.Empty(t, resp.Msg.KepubStatus)
}

func TestConnectGetKEPUBStatus_EPUBAndKEPUBReady(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertKEPUBRow(t, bookID, userID)

	req := connect.NewRequest(&booksv1.GetKEPUBStatusRequest{BookId: bookID.String()})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetKEPUBStatus(ctx, req)
	require.NoError(t, err)
	assert.True(t, resp.Msg.HasEpub)
	assert.Equal(t, models.FileStatusReady, resp.Msg.KepubStatus)
}

func TestConnectGetKEPUBStatus_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.GetKEPUBStatusRequest{BookId: "not-a-uuid"})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetKEPUBStatus(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestCreateKoboDevice_AndLookup(t *testing.T) {
	const isolatedUser = "kobo-device-repo-user-" // keep deterministic; unique test
	ctx := context.Background()
	uid := uuid.NewString()
	ownerID := isolatedUser + uid
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.kobo_devices WHERE user_id = $1`, ownerID)
	})

	hash := tokenHash("some-raw-token-value")
	device, err := testApp.Repositories.KoboDevices.CreateKoboDevice(
		ctx, ownerID, "My Kobo", "SN123", hash,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, device.ID)
	assert.Equal(t, ownerID, device.UserID)
	assert.Equal(t, "My Kobo", device.Name)
	assert.Equal(t, "SN123", device.Serial)
	assert.Nil(t, device.LastSeenAt)

	gotUserID, gotDeviceID, err := testApp.Repositories.KoboDevices.
		GetKoboAuthByTokenHash(ctx, hash)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotUserID)
	assert.Equal(t, device.ID, gotDeviceID)
}

func TestGetKoboAuthByTokenHash_NotFound(t *testing.T) {
	ctx := context.Background()
	_, _, err := testApp.Repositories.KoboDevices.GetKoboAuthByTokenHash(
		ctx, "nonexistent-hash-xyz",
	)
	require.Error(t, err)
	assert.True(t, errors.Is(err, database.ErrResourceNotFound))
}

func TestListKoboDevices_ReturnedInCreatedOrder(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-list-order-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.kobo_devices WHERE user_id = $1`, ownerID)
	})

	for i, name := range []string{"Device A", "Device B"} {
		_, err := testApp.Repositories.KoboDevices.CreateKoboDevice(
			ctx, ownerID, name, "", tokenHash("tok"+string(rune('a'+i))),
		)
		require.NoError(t, err)
	}

	devices, err := testApp.Repositories.KoboDevices.ListKoboDevices(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, devices, 2)
	assert.Equal(t, "Device A", devices[0].Name)
	assert.Equal(t, "Device B", devices[1].Name)
}

func TestDeleteKoboDevice_RevokesToken(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-delete-repo-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.kobo_devices WHERE user_id = $1`, ownerID)
	})

	hash := tokenHash("raw-token-to-revoke")
	device, err := testApp.Repositories.KoboDevices.CreateKoboDevice(
		ctx, ownerID, "Revoke Me", "", hash,
	)
	require.NoError(t, err)

	gotUID, _, err := testApp.Repositories.KoboDevices.GetKoboAuthByTokenHash(ctx, hash)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotUID)

	deviceID, err := uuid.Parse(device.ID)
	require.NoError(t, err)
	require.NoError(
		t,
		testApp.Repositories.KoboDevices.DeleteKoboDevice(ctx, ownerID, deviceID),
	)

	_, _, err = testApp.Repositories.KoboDevices.GetKoboAuthByTokenHash(ctx, hash)
	assert.True(t, errors.Is(err, database.ErrResourceNotFound),
		"token must be invalidated after device deletion")
}

func TestDeleteKoboDevice_WrongUser_NotFound(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-delete-wrong-owner-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.kobo_devices WHERE user_id = $1`, ownerID)
	})

	device, err := testApp.Repositories.KoboDevices.CreateKoboDevice(
		ctx, ownerID, "My Kobo", "", tokenHash("tok-xyz"),
	)
	require.NoError(t, err)

	deviceID, err := uuid.Parse(device.ID)
	require.NoError(t, err)

	err = testApp.Repositories.KoboDevices.DeleteKoboDevice(
		ctx, "someone-else-"+uuid.NewString(), deviceID,
	)
	assert.True(t, errors.Is(err, database.ErrResourceNotFound))
}

func TestGetKoboAuthByTokenHash_UpdatesLastSeenAt(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-lastseen-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.kobo_devices WHERE user_id = $1`, ownerID)
	})

	hash := tokenHash("tok-lastseen")
	device, err := testApp.Repositories.KoboDevices.CreateKoboDevice(
		ctx, ownerID, "Check LastSeen", "", hash,
	)
	require.NoError(t, err)
	assert.Nil(t, device.LastSeenAt, "last_seen_at must be nil before first auth")

	_, _, err = testApp.Repositories.KoboDevices.GetKoboAuthByTokenHash(ctx, hash)
	require.NoError(t, err)

	devices, err := testApp.Repositories.KoboDevices.ListKoboDevices(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.NotNil(t, devices[0].LastSeenAt, "last_seen_at must be set after first auth")
}

// TestListKoboSyncBooks_ReturnsConverterVersion checks converter_version is
// surfaced for stale detection.
func TestListKoboSyncBooks_ReturnsConverterVersion(t *testing.T) {
	ctx := context.Background()
	owner := "kobo-repo-converter-version-list-" + uuid.NewString()
	_, bookID := uploadFileForOwner(t, owner, models.FileFormatEPUB)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, owner, bookID))
	insertKEPUBRowWithVersion(t, bookID, owner, 3)

	books, err := testApp.Repositories.Books.ListKoboSyncBooks(ctx, owner)
	require.NoError(t, err)
	require.Len(t, books, 1)
	assert.Equal(t, int16(3), books[0].ConverterVersion)
}

func TestGetKoboSyncBook_ReturnsConverterVersion(t *testing.T) {
	ctx := context.Background()
	owner := "kobo-repo-converter-version-get-" + uuid.NewString()
	_, bookID := uploadFileForOwner(t, owner, models.FileFormatEPUB)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, owner, bookID))
	insertKEPUBRowWithVersion(t, bookID, owner, 4)

	book, err := testApp.Repositories.Books.GetKoboSyncBook(ctx, owner, bookID)
	require.NoError(t, err)
	assert.Equal(t, int16(4), book.ConverterVersion)
}

func TestRegisterKoboDevice_RawTokenNeverStoredAndLookupWorks(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-svc-device-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.kobo_devices WHERE user_id = $1`, ownerID)
	})

	device, rawToken, err := testApp.Services.Kobo.RegisterKoboDevice(
		ctx, ownerID, "My Kobo", "SN9999",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, rawToken)
	assert.NotEmpty(t, device.ID)

	var count int
	err = testDB.QueryRow(ctx,
		`SELECT COUNT(*) FROM books.kobo_devices
		 WHERE user_id = $1 AND token_hash = $2`,
		ownerID, rawToken,
	).Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count, "raw token must not be stored in the database")

	gotUserID, _, err := testApp.Services.Kobo.GetKoboAuthByTokenHash(
		ctx, tokenHash(rawToken),
	)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotUserID)
}

func TestRegisterKoboDevice_MultipleDevicesIndependent(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-multi-device-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.kobo_devices WHERE user_id = $1`, ownerID)
	})

	_, rawA, err := testApp.Services.Kobo.RegisterKoboDevice(
		ctx,
		ownerID,
		"Kobo A",
		"",
	)
	require.NoError(t, err)
	_, rawB, err := testApp.Services.Kobo.RegisterKoboDevice(
		ctx,
		ownerID,
		"Kobo B",
		"",
	)
	require.NoError(t, err)

	gotA, _, err := testApp.Services.Kobo.GetKoboAuthByTokenHash(
		ctx,
		tokenHash(rawA),
	)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotA)

	gotB, _, err := testApp.Services.Kobo.GetKoboAuthByTokenHash(
		ctx,
		tokenHash(rawB),
	)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotB)
}

// TestDisconnectKoboDevice_RevokesToken: a disconnected device's token gets 401.
func TestDisconnectKoboDevice_RevokesToken(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	ownerID := "kobo-revoke-e2e-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM books.kobo_devices WHERE user_id = $1`, ownerID)
	})

	device, rawToken, err := testApp.Services.Kobo.RegisterKoboDevice(
		ctx, ownerID, "Revoke E2E", "",
	)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(
		koboReq(t, http.MethodPost, koboURL(ts, rawToken, "/v1/initialization"), nil),
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(
		t,
		http.StatusOK,
		resp.StatusCode,
		"token must be valid before disconnect",
	)

	deviceID, err := uuid.Parse(device.ID)
	require.NoError(t, err)
	require.NoError(
		t,
		testApp.Services.Kobo.DisconnectKoboDevice(ctx, ownerID, deviceID),
	)

	resp2, err := http.DefaultClient.Do(
		koboReq(t, http.MethodPost, koboURL(ts, rawToken, "/v1/initialization"), nil),
	)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode,
		"token must be invalid (401) after disconnect")
}

func newKoboTestClient(t *testing.T) booksTestClient {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(testApp))
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL)
}

func TestConnectRegisterKoboDevice_ReturnsDeviceAndToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.RegisterKoboDeviceRequest{
		Name:   "My Kobo Touch",
		Serial: "N418ABCD1234",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := newKoboTestClient(t).RegisterKoboDevice(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.RawToken)
	require.NotNil(t, resp.Msg.Device)
	assert.Equal(t, "My Kobo Touch", resp.Msg.Device.Name)
	assert.Equal(t, "N418ABCD1234", resp.Msg.Device.Serial)
	assert.NotEmpty(t, resp.Msg.Device.Id)
}

func TestConnectRegisterKoboDevice_TokenLookupAfterRegister(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.RegisterKoboDeviceRequest{Name: "Lookup Test"})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := newKoboTestClient(t).RegisterKoboDevice(ctx, req)
	require.NoError(t, err)

	hash := tokenHash(resp.Msg.RawToken)
	gotUserID, _, err := testApp.Repositories.KoboDevices.GetKoboAuthByTokenHash(
		ctx, hash,
	)
	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
}

func TestConnectListKoboDevices_ReturnsRegisteredDevices(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := newKoboTestClient(t)

	for _, name := range []string{"Kobo 1", "Kobo 2"} {
		regReq := connect.NewRequest(&booksv1.RegisterKoboDeviceRequest{Name: name})
		regReq.Header().Set("Cookie", accessToken.String())
		_, err := client.RegisterKoboDevice(ctx, regReq)
		require.NoError(t, err)
	}

	listReq := connect.NewRequest(&booksv1.ListKoboDevicesRequest{})
	listReq.Header().Set("Cookie", accessToken.String())
	listResp, err := client.ListKoboDevices(ctx, listReq)
	require.NoError(t, err)
	// Other tests may have added more devices for userID.
	assert.GreaterOrEqual(t, len(listResp.Msg.Devices), 2)
}

func TestConnectDisconnectKoboDevice_RemovesDevice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := newKoboTestClient(t)

	regReq := connect.NewRequest(
		&booksv1.RegisterKoboDeviceRequest{Name: "To Remove"},
	)
	regReq.Header().Set("Cookie", accessToken.String())
	regResp, err := client.RegisterKoboDevice(ctx, regReq)
	require.NoError(t, err)
	deviceID := regResp.Msg.Device.Id

	discReq := connect.NewRequest(&booksv1.DisconnectKoboDeviceRequest{Id: deviceID})
	discReq.Header().Set("Cookie", accessToken.String())
	_, err = client.DisconnectKoboDevice(ctx, discReq)
	require.NoError(t, err)

	hash := tokenHash(regResp.Msg.RawToken)
	_, _, err = testApp.Repositories.KoboDevices.GetKoboAuthByTokenHash(ctx, hash)
	assert.True(t, errors.Is(err, database.ErrResourceNotFound),
		"token must be revoked after disconnect")
}

func TestConnectDisconnectKoboDevice_InvalidID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.DisconnectKoboDeviceRequest{Id: "not-a-uuid"})
	req.Header().Set("Cookie", accessToken.String())

	_, err := newKoboTestClient(t).DisconnectKoboDevice(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestConnectDisconnectKoboDevice_NotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.DisconnectKoboDeviceRequest{
		Id: uuid.NewString(),
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := newKoboTestClient(t).DisconnectKoboDevice(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestConnectRequestKEPUBConversion_PDFOnly_ReturnsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)

	req := connect.NewRequest(&booksv1.RequestKEPUBConversionRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RequestKEPUBConversion(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)

	// Must not set kobo-sync: this is a pure preview trigger.
	ub, err := testApp.Services.Books.GetUserBook(ctx, userID, bookID)
	require.NoError(t, err)
	assert.False(t, ub.HasTag(models.TagKoboSync),
		"RequestKEPUBConversion must not set the kobo-sync tag")
}

func TestConnectRequestKEPUBConversion_AlreadyReady_ReturnsReady(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertKEPUBRow(t, bookID, userID)

	req := connect.NewRequest(&booksv1.RequestKEPUBConversionRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RequestKEPUBConversion(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusReady, resp.Msg.KepubStatus)
}

// TestConnectRequestKEPUBConversion_StaleReadyKEPUB_ReturnsConverting: preview
// re-converts a stale KEPUB.
func TestConnectRequestKEPUBConversion_StaleReadyKEPUB_ReturnsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)
	insertStaleKEPUBRow(t, bookID, userID)

	req := connect.NewRequest(&booksv1.RequestKEPUBConversionRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RequestKEPUBConversion(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus,
		"a stale ready KEPUB must re-trigger conversion, not be served as-is")
}

func TestConnectRequestKEPUBConversion_PDFWithKoboFormatPDFTag_StillConverts(
	t *testing.T,
) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)
	err := testApp.Repositories.Books.UpdateTags(
		context.Background(), userID, bookID, []string{models.TagKoboFormatPDF},
		false, // no kobo-sync tag
	)
	require.NoError(t, err)

	req := connect.NewRequest(&booksv1.RequestKEPUBConversionRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	// Preview converts even when the sync preference is raw PDF.
	resp, err := client.RequestKEPUBConversion(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)
}

func TestConnectRequestKEPUBConversion_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&booksv1.RequestKEPUBConversionRequest{BookId: "bad-id"},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.RequestKEPUBConversion(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// TestUpdateTags_SetsKoboSyncEnabledAt checks enabling sets the timestamp.
func TestUpdateTags_SetsKoboSyncEnabledAt(t *testing.T) {
	ctx := context.Background()
	ub := addTestBook(t, "TagsEnabledAt-"+uuid.NewString())

	require.NoError(t, testApp.Repositories.Books.UpdateTags(
		ctx, userID, ub.BookID, []string{models.TagKoboSync},
		true,
	))

	var enabledAt *time.Time
	err := testDB.QueryRow(ctx,
		`SELECT kobo_sync_enabled_at
		   FROM books.user_books
		  WHERE user_id = $1 AND book_id = $2`,
		userID, ub.BookID,
	).Scan(&enabledAt)
	require.NoError(t, err)
	assert.NotNil(
		t,
		enabledAt,
		"kobo_sync_enabled_at must be set after enabling kobo-sync",
	)
}

// TestUpdateTags_PreservesKoboSyncEnabledAt checks other tag edits keep it.
func TestUpdateTags_PreservesKoboSyncEnabledAt(t *testing.T) {
	ctx := context.Background()
	ub := addTestBook(t, "TagsPreserveAt-"+uuid.NewString())

	require.NoError(t, testApp.Repositories.Books.UpdateTags(
		ctx, userID, ub.BookID, []string{models.TagKoboSync},
		true,
	))

	var first time.Time
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT kobo_sync_enabled_at
		   FROM books.user_books
		  WHERE user_id = $1 AND book_id = $2`,
		userID, ub.BookID,
	).Scan(&first))

	require.NoError(t, testApp.Repositories.Books.UpdateTags(
		ctx, userID, ub.BookID,
		[]string{models.TagKoboSync, models.TagKoboFormatPDF},
		true,
	))

	var second time.Time
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT kobo_sync_enabled_at
		   FROM books.user_books
		  WHERE user_id = $1 AND book_id = $2`,
		userID, ub.BookID,
	).Scan(&second))

	assert.True(t, first.Equal(second),
		"kobo_sync_enabled_at must not change when kobo-sync tag is kept")
}

// TestUpdateTags_ClearsKoboSyncEnabledAt checks removing kobo-sync clears it.
func TestUpdateTags_ClearsKoboSyncEnabledAt(t *testing.T) {
	ctx := context.Background()
	ub := addTestBook(t, "TagsClearAt-"+uuid.NewString())

	require.NoError(t, testApp.Repositories.Books.UpdateTags(
		ctx, userID, ub.BookID, []string{models.TagKoboSync},
		true,
	))
	require.NoError(t, testApp.Repositories.Books.UpdateTags(
		ctx, userID, ub.BookID, []string{},
		false,
	))

	var enabledAt *time.Time
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT kobo_sync_enabled_at
		   FROM books.user_books
		  WHERE user_id = $1 AND book_id = $2`,
		userID, ub.BookID,
	).Scan(&enabledAt))

	assert.Nil(t, enabledAt,
		"kobo_sync_enabled_at must be NULL after kobo-sync tag is removed")
}

func TestKoboRemoval_UpsertListDelete_RoundTrip(t *testing.T) {
	ctx := context.Background()
	book := addUniqueBook(t)
	owner := "kobo-removal-repo-" + uuid.NewString()

	require.NoError(t,
		testApp.Repositories.Books.UpsertKoboRemoval(ctx, owner, book.ID))

	removals, err := testApp.Repositories.Books.ListKoboRemovals(ctx, owner)
	require.NoError(t, err)
	require.Len(t, removals, 1)
	assert.Equal(t, book.ID, removals[0].BookID)

	require.NoError(t,
		testApp.Repositories.Books.DeleteKoboRemoval(ctx, owner, book.ID))

	removals, err = testApp.Repositories.Books.ListKoboRemovals(ctx, owner)
	require.NoError(t, err)
	assert.Empty(t, removals)
}

func TestKoboRemoval_Upsert_IdempotentOnConflict(t *testing.T) {
	ctx := context.Background()
	book := addUniqueBook(t)
	owner := "kobo-removal-idempotent-" + uuid.NewString()

	require.NoError(t,
		testApp.Repositories.Books.UpsertKoboRemoval(ctx, owner, book.ID))
	require.NoError(t,
		testApp.Repositories.Books.UpsertKoboRemoval(ctx, owner, book.ID))

	removals, err := testApp.Repositories.Books.ListKoboRemovals(ctx, owner)
	require.NoError(t, err)
	require.Len(t, removals, 1, "re-upserting the same book must not duplicate")
}

// TestToggleTag_DisableKoboSync_WritesTombstone checks disabling tombstones the book.
func TestToggleTag_DisableKoboSync_WritesTombstone(t *testing.T) {
	ctx := context.Background()
	ub := addTestBook(t, "KoboRemovalDisable-"+uuid.NewString())

	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, userID, ub.BookID))
	require.NoError(t,
		testApp.Services.Books.ToggleTag(ctx, userID, ub.BookID, models.TagKoboSync))

	removals, err := testApp.Services.Books.ListKoboRemovals(ctx, userID)
	require.NoError(t, err)
	found := false
	for _, r := range removals {
		if r.BookID == ub.BookID {
			found = true
		}
	}
	assert.True(t, found, "disabling kobo-sync must tombstone the book for removal")
}

// TestEnableKoboSync_ClearsStaleTombstone checks re-enabling clears it.
func TestEnableKoboSync_ClearsStaleTombstone(t *testing.T) {
	ctx := context.Background()
	ub := addTestBook(t, "KoboRemovalReenable-"+uuid.NewString())

	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, userID, ub.BookID))
	require.NoError(t,
		testApp.Services.Books.ToggleTag(ctx, userID, ub.BookID, models.TagKoboSync))
	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, userID, ub.BookID))

	removals, err := testApp.Services.Books.ListKoboRemovals(ctx, userID)
	require.NoError(t, err)
	for _, r := range removals {
		assert.NotEqual(t, ub.BookID, r.BookID,
			"re-enabling kobo-sync must clear the stale removal tombstone")
	}
}

// TestToggleTag_ReenableViaToggle_ClearsTombstone covers ToggleTag's own clear.
func TestToggleTag_ReenableViaToggle_ClearsTombstone(t *testing.T) {
	ctx := context.Background()
	ub := addTestBook(t, "KoboRemovalToggleReenable-"+uuid.NewString())

	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, userID, ub.BookID))
	require.NoError(t, // disable
		testApp.Services.Books.ToggleTag(ctx, userID, ub.BookID, models.TagKoboSync))
	require.NoError(t, // re-enable via ToggleTag, not EnableKoboSync
		testApp.Services.Books.ToggleTag(ctx, userID, ub.BookID, models.TagKoboSync))

	removals, err := testApp.Services.Books.ListKoboRemovals(ctx, userID)
	require.NoError(t, err)
	for _, r := range removals {
		assert.NotEqual(t, ub.BookID, r.BookID,
			"re-enabling via ToggleTag must clear the tombstone")
	}
}

// TestRemoveFromLibrary_NoUserBookRow_NoError tolerates a missing user_books row.
func TestRemoveFromLibrary_NoUserBookRow_NoError(t *testing.T) {
	ctx := context.Background()
	owner := "kobo-removal-no-userbook-" + uuid.NewString()
	book := addUniqueBook(t)

	err := testApp.Services.Books.RemoveFromLibrary(ctx, owner, book.ID)
	require.NoError(t, err,
		"removing a book with no user_books row must not error")
}

// TestToggleTag_UnrelatedTag_DoesNotTombstone checks other tags leave tombstones alone.
func TestToggleTag_UnrelatedTag_DoesNotTombstone(t *testing.T) {
	ctx := context.Background()
	ub := addTestBook(t, "KoboRemovalUnrelated-"+uuid.NewString())

	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, userID, ub.BookID))
	require.NoError(
		t,
		testApp.Services.Books.ToggleTag(
			ctx,
			userID,
			ub.BookID,
			models.TagKoboFormatPDF,
		),
	)

	removals, err := testApp.Services.Books.ListKoboRemovals(ctx, userID)
	require.NoError(t, err)
	for _, r := range removals {
		assert.NotEqual(t, ub.BookID, r.BookID,
			"toggling an unrelated tag must not tombstone the book")
	}
}

// TestRemoveFromLibrary_KoboSyncedBook_WritesTombstone checks deleting a synced
// book tombstones it.
func TestRemoveFromLibrary_KoboSyncedBook_WritesTombstone(t *testing.T) {
	ctx := context.Background()
	owner := "kobo-removal-delete-" + uuid.NewString()
	_, bookID := uploadFileForOwner(t, owner, models.FileFormatEPUB)
	require.NoError(t, testApp.Services.Books.EnableKoboSync(ctx, owner, bookID))

	require.NoError(t, testApp.Services.Books.RemoveFromLibrary(ctx, owner, bookID))

	removals, err := testApp.Services.Books.ListKoboRemovals(ctx, owner)
	require.NoError(t, err)
	found := false
	for _, r := range removals {
		if r.BookID == bookID {
			found = true
		}
	}
	assert.True(t, found,
		"deleting a kobo-synced book must tombstone it for removal")
}

// TestRemoveFromLibrary_NonSyncedBook_NoTombstone checks no spurious tombstone.
func TestRemoveFromLibrary_NonSyncedBook_NoTombstone(t *testing.T) {
	ctx := context.Background()
	owner := "kobo-removal-delete-nosync-" + uuid.NewString()
	_, bookID := uploadFileForOwner(t, owner, models.FileFormatEPUB)

	require.NoError(t, testApp.Services.Books.RemoveFromLibrary(ctx, owner, bookID))

	removals, err := testApp.Services.Books.ListKoboRemovals(ctx, owner)
	require.NoError(t, err)
	for _, r := range removals {
		assert.NotEqual(t, bookID, r.BookID,
			"deleting a never-synced book must not tombstone it")
	}
}

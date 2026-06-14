package backlog_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
	"tools.xdoubleu.com/internal/testhelper"
)

// tokenHash hashes a raw token the same way the service does (sha256 hex).
func tokenHash(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// --- service-level tests for GetKEPUBStatus ---

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
}

func TestGetKEPUBStatus_EPUBAndKEPUBConverting(t *testing.T) {
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)

	// Insert a converting KEPUB row (simulates an in-progress conversion).
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

// --- service-level tests for GetKoboFileFormat ---

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
	)
	require.NoError(t, err)

	format, err := testApp.Services.Books.GetKoboFileFormat(
		context.Background(), userID, ub.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.FileFormatPDF, format)
}

// --- service-level tests for EnableKoboSync ---

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

// --- handler-level tests ---

func TestConnectEnableKoboSync_EPUBBook_ReturnsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatEPUB)

	req := connect.NewRequest(&backlogv1.EnableKoboSyncRequest{
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

	req := connect.NewRequest(&backlogv1.EnableKoboSyncRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.EnableKoboSync(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusReady, resp.Msg.KepubStatus)
}

func TestConnectEnableKoboSync_PDFOnly_ReturnsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)

	req := connect.NewRequest(&backlogv1.EnableKoboSyncRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.EnableKoboSync(ctx, req)
	require.NoError(t, err)
	// PDF-only books default to wanting KEPUB, so conversion is triggered.
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)
}

func TestConnectEnableKoboSync_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.EnableKoboSyncRequest{BookId: "bad-id"})
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
		&backlogv1.GetKEPUBStatusRequest{BookId: book.ID.String()},
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

	req := connect.NewRequest(&backlogv1.GetKEPUBStatusRequest{BookId: bookID.String()})
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

	req := connect.NewRequest(&backlogv1.GetKEPUBStatusRequest{BookId: "not-a-uuid"})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetKEPUBStatus(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// --- Kobo device: repo tests ---

func TestCreateKoboDevice_AndLookup(t *testing.T) {
	const isolatedUser = "kobo-device-repo-user-" // keep deterministic; unique test
	ctx := context.Background()
	uid := uuid.NewString()
	ownerID := isolatedUser + uid
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.kobo_devices WHERE user_id = $1`, ownerID)
	})

	hash := tokenHash("some-raw-token-value")
	device, err := testApp.Repositories.Integrations.CreateKoboDevice(
		ctx, ownerID, "My Kobo", "SN123", hash,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, device.ID)
	assert.Equal(t, ownerID, device.UserID)
	assert.Equal(t, "My Kobo", device.Name)
	assert.Equal(t, "SN123", device.Serial)
	assert.Nil(t, device.LastSeenAt)

	gotUserID, err := testApp.Repositories.Integrations.GetUserIDByKoboTokenHash(
		ctx,
		hash,
	)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotUserID)
}

func TestGetUserIDByKoboTokenHash_NotFound(t *testing.T) {
	ctx := context.Background()
	_, err := testApp.Repositories.Integrations.GetUserIDByKoboTokenHash(
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
			`DELETE FROM backlog.kobo_devices WHERE user_id = $1`, ownerID)
	})

	for i, name := range []string{"Device A", "Device B"} {
		_, err := testApp.Repositories.Integrations.CreateKoboDevice(
			ctx, ownerID, name, "", tokenHash("tok"+string(rune('a'+i))),
		)
		require.NoError(t, err)
	}

	devices, err := testApp.Repositories.Integrations.ListKoboDevices(ctx, ownerID)
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
			`DELETE FROM backlog.kobo_devices WHERE user_id = $1`, ownerID)
	})

	hash := tokenHash("raw-token-to-revoke")
	device, err := testApp.Repositories.Integrations.CreateKoboDevice(
		ctx, ownerID, "Revoke Me", "", hash,
	)
	require.NoError(t, err)

	// Token must be valid before deletion.
	gotUID, err := testApp.Repositories.Integrations.GetUserIDByKoboTokenHash(ctx, hash)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotUID)

	deviceID, err := uuid.Parse(device.ID)
	require.NoError(t, err)
	require.NoError(
		t,
		testApp.Repositories.Integrations.DeleteKoboDevice(ctx, ownerID, deviceID),
	)

	// Token must be invalid (not found) after deletion — TDD: this fails before
	// DeleteKoboDevice is implemented correctly.
	_, err = testApp.Repositories.Integrations.GetUserIDByKoboTokenHash(ctx, hash)
	assert.True(t, errors.Is(err, database.ErrResourceNotFound),
		"token must be invalidated after device deletion")
}

func TestDeleteKoboDevice_WrongUser_NotFound(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-delete-wrong-owner-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.kobo_devices WHERE user_id = $1`, ownerID)
	})

	device, err := testApp.Repositories.Integrations.CreateKoboDevice(
		ctx, ownerID, "My Kobo", "", tokenHash("tok-xyz"),
	)
	require.NoError(t, err)

	deviceID, err := uuid.Parse(device.ID)
	require.NoError(t, err)

	// Different user tries to delete the device.
	err = testApp.Repositories.Integrations.DeleteKoboDevice(
		ctx, "someone-else-"+uuid.NewString(), deviceID,
	)
	assert.True(t, errors.Is(err, database.ErrResourceNotFound))
}

func TestGetUserIDByKoboTokenHash_UpdatesLastSeenAt(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-lastseen-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.kobo_devices WHERE user_id = $1`, ownerID)
	})

	hash := tokenHash("tok-lastseen")
	device, err := testApp.Repositories.Integrations.CreateKoboDevice(
		ctx, ownerID, "Check LastSeen", "", hash,
	)
	require.NoError(t, err)
	assert.Nil(t, device.LastSeenAt, "last_seen_at must be nil before first auth")

	// Authenticate — should touch last_seen_at.
	_, err = testApp.Repositories.Integrations.GetUserIDByKoboTokenHash(ctx, hash)
	require.NoError(t, err)

	devices, err := testApp.Repositories.Integrations.ListKoboDevices(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.NotNil(t, devices[0].LastSeenAt, "last_seen_at must be set after first auth")
}

// --- Kobo device: service tests ---

func TestRegisterKoboDevice_RawTokenNeverStoredAndLookupWorks(t *testing.T) {
	ctx := context.Background()
	ownerID := "kobo-svc-device-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.kobo_devices WHERE user_id = $1`, ownerID)
	})

	device, rawToken, err := testApp.Services.Integrations.RegisterKoboDevice(
		ctx, ownerID, "My Kobo", "SN9999",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, rawToken)
	assert.NotEmpty(t, device.ID)

	// Raw token must not appear verbatim in the DB.
	var count int
	err = testDB.QueryRow(ctx,
		`SELECT COUNT(*) FROM backlog.kobo_devices
		 WHERE user_id = $1 AND token_hash = $2`,
		ownerID, rawToken,
	).Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count, "raw token must not be stored in the database")

	// Hash of the raw token must resolve to the user.
	gotUserID, err := testApp.Services.Integrations.GetUserIDByKoboTokenHash(
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
			`DELETE FROM backlog.kobo_devices WHERE user_id = $1`, ownerID)
	})

	_, rawA, err := testApp.Services.Integrations.RegisterKoboDevice(
		ctx,
		ownerID,
		"Kobo A",
		"",
	)
	require.NoError(t, err)
	_, rawB, err := testApp.Services.Integrations.RegisterKoboDevice(
		ctx,
		ownerID,
		"Kobo B",
		"",
	)
	require.NoError(t, err)

	// Both tokens must independently resolve to the same user.
	gotA, err := testApp.Services.Integrations.GetUserIDByKoboTokenHash(
		ctx,
		tokenHash(rawA),
	)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotA)

	gotB, err := testApp.Services.Integrations.GetUserIDByKoboTokenHash(
		ctx,
		tokenHash(rawB),
	)
	require.NoError(t, err)
	assert.Equal(t, ownerID, gotB)
}

// TestDisconnectKoboDevice_RevokesToken is the TDD anchor for the revoke path:
// it is written before the disconnect RPC existed, asserting that after a
// device is disconnected its sync token returns 401 from the Kobo sync route.
func TestDisconnectKoboDevice_RevokesToken(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)

	ownerID := "kobo-revoke-e2e-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.kobo_devices WHERE user_id = $1`, ownerID)
	})

	device, rawToken, err := testApp.Services.Integrations.RegisterKoboDevice(
		ctx, ownerID, "Revoke E2E", "",
	)
	require.NoError(t, err)

	// Token must work before disconnect.
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
		testApp.Services.Integrations.DisconnectKoboDevice(ctx, ownerID, deviceID),
	)

	// Token must be rejected after disconnect.
	resp2, err := http.DefaultClient.Do(
		koboReq(t, http.MethodPost, koboURL(ts, rawToken, "/v1/initialization"), nil),
	)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode,
		"token must be invalid (401) after disconnect")
}

// --- Kobo device: connect handler tests ---

func newKoboTestClient(t *testing.T) backlogv1connect.BooksServiceClient {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(testApp))
	t.Cleanup(ts.Close)
	return backlogv1connect.NewBooksServiceClient(http.DefaultClient, ts.URL)
}

func TestConnectRegisterKoboDevice_ReturnsDeviceAndToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.RegisterKoboDeviceRequest{
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

	req := connect.NewRequest(&backlogv1.RegisterKoboDeviceRequest{Name: "Lookup Test"})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := newKoboTestClient(t).RegisterKoboDevice(ctx, req)
	require.NoError(t, err)

	// The returned token must resolve to the correct user.
	hash := tokenHash(resp.Msg.RawToken)
	gotUserID, err := testApp.Repositories.Integrations.GetUserIDByKoboTokenHash(
		ctx, hash,
	)
	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
}

func TestConnectListKoboDevices_ReturnsRegisteredDevices(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := newKoboTestClient(t)

	// Register two devices.
	for _, name := range []string{"Kobo 1", "Kobo 2"} {
		regReq := connect.NewRequest(&backlogv1.RegisterKoboDeviceRequest{Name: name})
		regReq.Header().Set("Cookie", accessToken.String())
		_, err := client.RegisterKoboDevice(ctx, regReq)
		require.NoError(t, err)
	}

	listReq := connect.NewRequest(&backlogv1.ListKoboDevicesRequest{})
	listReq.Header().Set("Cookie", accessToken.String())
	listResp, err := client.ListKoboDevices(ctx, listReq)
	require.NoError(t, err)
	// At least the two we registered; other tests may have added more for userID.
	assert.GreaterOrEqual(t, len(listResp.Msg.Devices), 2)
}

func TestConnectDisconnectKoboDevice_RemovesDevice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := newKoboTestClient(t)

	// Register a device to disconnect.
	regReq := connect.NewRequest(
		&backlogv1.RegisterKoboDeviceRequest{Name: "To Remove"},
	)
	regReq.Header().Set("Cookie", accessToken.String())
	regResp, err := client.RegisterKoboDevice(ctx, regReq)
	require.NoError(t, err)
	deviceID := regResp.Msg.Device.Id

	// Disconnect it.
	discReq := connect.NewRequest(&backlogv1.DisconnectKoboDeviceRequest{Id: deviceID})
	discReq.Header().Set("Cookie", accessToken.String())
	_, err = client.DisconnectKoboDevice(ctx, discReq)
	require.NoError(t, err)

	// Token must now be invalid.
	hash := tokenHash(regResp.Msg.RawToken)
	_, err = testApp.Repositories.Integrations.GetUserIDByKoboTokenHash(ctx, hash)
	assert.True(t, errors.Is(err, database.ErrResourceNotFound),
		"token must be revoked after disconnect")
}

func TestConnectDisconnectKoboDevice_InvalidID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.DisconnectKoboDeviceRequest{Id: "not-a-uuid"})
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

	req := connect.NewRequest(&backlogv1.DisconnectKoboDeviceRequest{
		Id: uuid.NewString(),
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := newKoboTestClient(t).DisconnectKoboDevice(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

// --- RequestKEPUBConversion handler tests ---

func TestConnectRequestKEPUBConversion_PDFOnly_ReturnsConverting(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)

	req := connect.NewRequest(&backlogv1.RequestKEPUBConversionRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RequestKEPUBConversion(ctx, req)
	require.NoError(t, err)
	// Conversion must be triggered regardless of kobo-format-pdf preference.
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)

	// Must NOT set the kobo-sync tag — this is a pure preview trigger.
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

	req := connect.NewRequest(&backlogv1.RequestKEPUBConversionRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RequestKEPUBConversion(ctx, req)
	require.NoError(t, err)
	// KEPUB already ready — no new conversion should start.
	assert.Equal(t, models.FileStatusReady, resp.Msg.KepubStatus)
}

func TestConnectRequestKEPUBConversion_PDFWithKoboFormatPDFTag_StillConverts(
	t *testing.T,
) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Upload a PDF and tag the book with kobo-format-pdf (user prefers raw PDF for sync).
	_, bookID := uploadFileForOwner(t, userID, models.FileFormatPDF)
	err := testApp.Repositories.Books.UpdateTags(
		context.Background(), userID, bookID, []string{models.TagKoboFormatPDF},
	)
	require.NoError(t, err)

	req := connect.NewRequest(&backlogv1.RequestKEPUBConversionRequest{
		BookId: bookID.String(),
	})
	req.Header().Set("Cookie", accessToken.String())

	// Even though the user's Kobo sync preference is "raw PDF", preview must
	// still trigger conversion so the user can judge the output.
	resp, err := client.RequestKEPUBConversion(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, models.FileStatusConverting, resp.Msg.KepubStatus)
}

func TestConnectRequestKEPUBConversion_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.RequestKEPUBConversionRequest{BookId: "bad-id"},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.RequestKEPUBConversion(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

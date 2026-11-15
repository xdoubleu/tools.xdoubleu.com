package zitadel_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/platform/zitadel"
)

func cfgFor(issuer, token string) config.Config {
	cfg := testhelper.NewTestConfig()
	cfg.OIDCIssuer = issuer
	cfg.ZitadelServiceToken = token
	return cfg
}

// fakeZitadel records the calls user management makes.
type fakeZitadel struct {
	calls  []string
	bodies map[string]map[string]any
	status map[string]int
}

func (f *fakeZitadel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	f.calls = append(f.calls, key)
	if r.Header.Get("Authorization") != "Bearer pat" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.bodies[key] = body
	if code, ok := f.status[key]; ok {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
		return
	}
	if key == "POST /v2/users" {
		_, _ = w.Write([]byte(`{"result":[{"userId":"zid-existing"}]}`))
		return
	}
	if key == "POST /v2/users/human" {
		_, _ = w.Write([]byte(`{"userId":"zid-1"}`))
		return
	}
	_, _ = w.Write([]byte(`{}`))
}

func newClient(t *testing.T) (*zitadel.Client, *fakeZitadel) {
	t.Helper()
	fake := &fakeZitadel{
		calls:  nil,
		bodies: map[string]map[string]any{},
		status: map[string]int{},
	}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	client := zitadel.New(cfgFor(srv.URL+"/", "pat"))
	require.NotNil(t, client)
	return client, fake
}

func TestNewIsNilWithoutToken(t *testing.T) {
	assert.Nil(t, zitadel.New(cfgFor("https://id.example.com", "")))
	assert.Nil(t, zitadel.New(cfgFor("", "pat")))
}

func TestCreateHumanSendsVerifiedEmailAndNames(t *testing.T) {
	client, fake := newClient(t)

	id, err := client.CreateHuman(
		context.Background(), "ada@example.com", "Ada King Lovelace",
	)
	require.NoError(t, err)
	assert.Equal(t, "zid-1", id)

	body := fake.bodies["POST /v2/users/human"]
	assert.Equal(t, "ada@example.com", body["username"])
	assert.Equal(t,
		map[string]any{"email": "ada@example.com", "isVerified": true}, body["email"])
	assert.Equal(t, map[string]any{
		"givenName": "Ada", "familyName": "King Lovelace", "displayName": "Ada King Lovelace",
	}, body["profile"])
}

func TestCreateHumanFallsBackToEmailNames(t *testing.T) {
	client, fake := newClient(t)

	_, err := client.CreateHuman(context.Background(), "ada@example.com", "")
	require.NoError(t, err)
	profile, ok := fake.bodies["POST /v2/users/human"]["profile"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ada", profile["givenName"])
	assert.Equal(t, "-", profile["familyName"])

	_, err = client.CreateHuman(context.Background(), "b@example.com", "Bob")
	require.NoError(t, err)
	profile, ok = fake.bodies["POST /v2/users/human"]["profile"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Bob", profile["givenName"])
	assert.Equal(t, "-", profile["familyName"])
}

func TestCreateHumanFailures(t *testing.T) {
	client, fake := newClient(t)
	fake.status["POST /v2/users/human"] = http.StatusConflict

	_, err := client.CreateHuman(context.Background(), "ada@example.com", "")
	var status *zitadel.StatusError
	require.ErrorAs(t, err, &status)
	assert.Equal(t, http.StatusConflict, status.Code)
	assert.Contains(t, status.Error(), "nope")
}

func TestCreateHumanWithoutID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) },
	))
	t.Cleanup(srv.Close)
	client := zitadel.New(cfgFor(srv.URL, "pat"))

	_, err := client.CreateHuman(context.Background(), "ada@example.com", "")
	require.Error(t, err)
}

func TestRejectsBadToken(t *testing.T) {
	_, fake := newClient(t)
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	client := zitadel.New(cfgFor(srv.URL, "wrong"))

	_, err := client.CreateHuman(context.Background(), "ada@example.com", "")
	require.Error(t, err)
}

func TestInviteDeactivateDelete(t *testing.T) {
	client, fake := newClient(t)
	ctx := context.Background()

	require.NoError(t, client.SendInvite(ctx, "zid-1"))
	require.NoError(t, client.Deactivate(ctx, "zid-1"))
	require.NoError(t, client.Delete(ctx, "zid-1"))

	assert.Equal(t, []string{
		"POST /v2/users/zid-1/invite_code",
		"POST /v2/users/zid-1/deactivate",
		"DELETE /v2/users/zid-1",
	}, fake.calls)
	assert.Equal(t, map[string]any{"sendCode": map[string]any{}},
		fake.bodies["POST /v2/users/zid-1/invite_code"])
}

func TestRepeatsAreNotErrors(t *testing.T) {
	client, fake := newClient(t)
	fake.status["POST /v2/users/zid-1/deactivate"] = http.StatusPreconditionFailed
	fake.status["DELETE /v2/users/zid-1"] = http.StatusNotFound

	require.NoError(t, client.Deactivate(context.Background(), "zid-1"))
	require.NoError(t, client.Delete(context.Background(), "zid-1"))
}

func TestFailuresSurface(t *testing.T) {
	client, fake := newClient(t)
	fake.status["POST /v2/users/zid-1/invite_code"] = http.StatusInternalServerError
	fake.status["POST /v2/users/zid-1/deactivate"] = http.StatusForbidden
	fake.status["DELETE /v2/users/zid-1"] = http.StatusForbidden

	require.Error(t, client.SendInvite(context.Background(), "zid-1"))
	require.Error(t, client.Deactivate(context.Background(), "zid-1"))
	require.Error(t, client.Delete(context.Background(), "zid-1"))
}

func TestUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	client := zitadel.New(cfgFor(srv.URL, "pat"))

	require.Error(t, client.SendInvite(context.Background(), "x"))
}

func TestImportHumanCarriesCredentials(t *testing.T) {
	client, fake := newClient(t)

	id, err := client.ImportHuman(
		context.Background(), "ada@example.com", "Ada L", "$2a$10$hash", "BASE32",
	)

	require.NoError(t, err)
	assert.Equal(t, "zid-1", id)
	body := fake.bodies["POST /v2/users/human"]
	assert.Equal(t, "BASE32", body["totpSecret"])
	assert.Equal(t, map[string]any{
		"hashedPassword": map[string]any{"hash": "$2a$10$hash"},
	}, body["password"])
}

func TestImportHumanWithoutTOTP(t *testing.T) {
	client, fake := newClient(t)

	_, err := client.ImportHuman(
		context.Background(), "ada@example.com", "", "$2a$10$hash", "",
	)

	require.NoError(t, err)
	assert.NotContains(t, fake.bodies["POST /v2/users/human"], "totpSecret")
}

func TestImportHumanLinksExistingUser(t *testing.T) {
	client, fake := newClient(t)
	fake.status["POST /v2/users/human"] = http.StatusConflict
	delete(fake.status, "POST /v2/users")

	id, err := client.ImportHuman(
		context.Background(), "ada@example.com", "", "$2a$10$hash", "",
	)

	require.NoError(t, err)
	assert.Equal(t, "zid-existing", id)
}

func TestImportHumanFailure(t *testing.T) {
	client, fake := newClient(t)
	fake.status["POST /v2/users/human"] = http.StatusBadRequest

	_, err := client.ImportHuman(
		context.Background(), "ada@example.com", "", "bad", "",
	)

	require.Error(t, err)
}

func TestImportHumanConflictWithoutMatch(t *testing.T) {
	client, fake := newClient(t)
	fake.status["POST /v2/users/human"] = http.StatusConflict
	fake.status["POST /v2/users"] = http.StatusInternalServerError

	_, err := client.ImportHuman(
		context.Background(), "ada@example.com", "", "h", "",
	)

	require.Error(t, err)
}

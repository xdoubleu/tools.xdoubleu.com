package factory_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/testhelper"
)

// fakeStepUp passes or fails the step-up check and remembers what it was given.
type fakeStepUp struct {
	err         error
	token, code string
}

func (f *fakeStepUp) VerifyCurrentFactor(_ context.Context, token, code string) error {
	f.token, f.code = token, code
	return f.err
}

// stepUpReturning is a step-up check that fails with err (nil passes).
func stepUpReturning(err error) *fakeStepUp {
	return &fakeStepUp{err: err, token: "", code: ""}
}

func cleanRunners(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), "DELETE FROM factory.runners")
	require.NoError(t, err)
}

// runnerEnv serves both runner services of one app.
type runnerEnv struct {
	admin  factoryv1connect.RunnerAdminServiceClient
	runner factoryv1connect.RunnerServiceClient
}

func newRunnerEnv(
	t *testing.T,
	asAdmin bool,
	channel string,
	opts ...factory.Option,
) runnerEnv {
	t.Helper()
	cleanRunners(t)
	ts := httptest.NewServer(testhelper.BuildMux(newAppWith(asAdmin, channel, opts...)))
	t.Cleanup(ts.Close)
	transport := connecthttp.NewTransport(
		ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0),
	)
	return runnerEnv{
		admin:  factoryv1connect.NewRunnerAdminServiceClient(connect.NewClient(transport)),
		runner: factoryv1connect.NewRunnerServiceClient(connect.NewClient(transport)),
	}
}

func browserRunnerEnv(t *testing.T, opts ...factory.Option) runnerEnv {
	t.Helper()
	opts = append([]factory.Option{
		factory.WithStepUp(stepUpReturning(nil)),
		factory.WithHeartbeatHold(10 * time.Millisecond),
	}, opts...)
	return newRunnerEnv(t, true, constants.ChannelBrowser, opts...)
}

// signedIn is a context carrying the session cookie the step-up check reads.
func signedIn() context.Context {
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set("Cookie", "accessToken=session-jwt")
	return ctx
}

func bearer(token string) context.Context {
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set("Authorization", "Bearer "+token)
	return ctx
}

func createRunner(
	t *testing.T,
	env runnerEnv,
	name string,
) *factoryv1.CreateRunnerResponse {
	t.Helper()
	resp, err := env.admin.CreateRunner(signedIn(),
		&factoryv1.CreateRunnerRequest{Name: name, TotpCode: "123456"})
	require.NoError(t, err)
	return resp
}

func pair(t *testing.T, env runnerEnv, code string) *factoryv1.PairResponse {
	t.Helper()
	resp, err := env.runner.Pair(context.Background(),
		&factoryv1.PairRequest{Code: code, Version: "1.2.3"})
	require.NoError(t, err)
	return resp
}

func listRunners(t *testing.T, env runnerEnv) []*factoryv1.Runner {
	t.Helper()
	resp, err := env.admin.ListRunners(signedIn(), &factoryv1.ListRunnersRequest{})
	require.NoError(t, err)
	return resp.Runners
}

func TestRunnerPairsHeartbeatsAndIsRevoked(t *testing.T) {
	stepUp := stepUpReturning(nil)
	env := browserRunnerEnv(t, factory.WithStepUp(stepUp))

	created := createRunner(t, env, "  Home Mac ")
	assert.Equal(t, "Home Mac", created.Runner.Name)
	assert.Regexp(t, regexp.MustCompile(`^[0-9A-Z]{5}-[0-9A-Z]{5}$`), created.PairingCode)
	assert.Equal(t, "session-jwt", stepUp.token)
	assert.Equal(t, "123456", stepUp.code)
	assert.Empty(t, created.Runner.PairedAt)
	assert.False(t, created.Runner.Online)
	expires, err := time.Parse(time.RFC3339, created.Runner.PairingExpiresAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(10*time.Minute), expires, time.Minute)

	// A code typed in lower case and without the dash still works.
	typed := strings.ToLower(strings.ReplaceAll(created.PairingCode, "-", ""))
	paired := pair(t, env, typed)
	assert.True(t, strings.HasPrefix(paired.DeviceToken, "frt_"))
	assert.Equal(t, "Home Mac", paired.RunnerName)

	runners := listRunners(t, env)
	require.Len(t, runners, 1)
	assert.Equal(t, "1.2.3", runners[0].Version)
	assert.NotEmpty(t, runners[0].PairedAt)
	assert.Empty(t, runners[0].PairingExpiresAt)
	assert.True(t, runners[0].Online)

	_, err = env.runner.Heartbeat(bearer(paired.DeviceToken),
		&factoryv1.HeartbeatRequest{Version: "1.3.0"})
	require.NoError(t, err)
	assert.Equal(t, "1.3.0", listRunners(t, env)[0].Version)

	revoked, err := env.admin.RevokeRunner(signedIn(),
		&factoryv1.RevokeRunnerRequest{Id: runners[0].Id})
	require.NoError(t, err)
	assert.NotEmpty(t, revoked.Runner.RevokedAt)
	assert.False(t, revoked.Runner.Online)

	_, err = env.runner.Heartbeat(bearer(paired.DeviceToken),
		&factoryv1.HeartbeatRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	again, err := env.admin.RevokeRunner(signedIn(),
		&factoryv1.RevokeRunnerRequest{Id: runners[0].Id})
	require.NoError(t, err)
	assert.Equal(t, revoked.Runner.RevokedAt, again.Runner.RevokedAt)
}

func TestOnlyHashesAreStored(t *testing.T) {
	env := browserRunnerEnv(t)
	created := createRunner(t, env, "Home Mac")
	paired := pair(t, env, created.PairingCode)

	var tokenHash string
	var pairingHash *string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT token_hash, pairing_hash FROM factory.runners WHERE id = $1`,
		created.Runner.Id).Scan(&tokenHash, &pairingHash))
	assert.Len(t, tokenHash, 64)
	assert.NotContains(t, tokenHash, paired.DeviceToken)
	assert.Nil(t, pairingHash, "a used code is cleared")
}

func TestAPairingCodeWorksOnce(t *testing.T) {
	env := browserRunnerEnv(t)
	created := createRunner(t, env, "Home Mac")
	pair(t, env, created.PairingCode)

	_, err := env.runner.Pair(context.Background(),
		&factoryv1.PairRequest{Code: created.PairingCode})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestAnExpiredOrWrongCodeIsRefused(t *testing.T) {
	env := browserRunnerEnv(t)
	created := createRunner(t, env, "Home Mac")
	_, err := testDB.Exec(context.Background(), `
		UPDATE factory.runners SET pairing_expires_at = now() - interval '1 minute'
		WHERE id = $1`, created.Runner.Id)
	require.NoError(t, err)

	_, err = env.runner.Pair(context.Background(),
		&factoryv1.PairRequest{Code: created.PairingCode})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "expired")

	_, err = env.runner.Pair(context.Background(),
		&factoryv1.PairRequest{Code: "AAAAA-BBBBB"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "unknown")

	_, err = env.runner.Pair(context.Background(),
		&factoryv1.PairRequest{Code: "short"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "malformed")
}

func TestARevokedRunnersCodeIsRefused(t *testing.T) {
	env := browserRunnerEnv(t)
	created := createRunner(t, env, "Home Mac")
	_, err := env.admin.RevokeRunner(signedIn(),
		&factoryv1.RevokeRunnerRequest{Id: created.Runner.Id})
	require.NoError(t, err)

	_, err = env.runner.Pair(context.Background(),
		&factoryv1.PairRequest{Code: created.PairingCode})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestOnlyOneRunnerCanBeActive(t *testing.T) {
	env := browserRunnerEnv(t)
	first := createRunner(t, env, "First")

	_, err := env.admin.CreateRunner(signedIn(),
		&factoryv1.CreateRunnerRequest{Name: "Second", TotpCode: "1"})
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err), "still pairing")

	pair(t, env, first.PairingCode)
	_, err = env.admin.CreateRunner(signedIn(),
		&factoryv1.CreateRunnerRequest{Name: "Second", TotpCode: "1"})
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err), "paired")

	_, err = env.admin.RevokeRunner(signedIn(),
		&factoryv1.RevokeRunnerRequest{Id: first.Runner.Id})
	require.NoError(t, err)
	createRunner(t, env, "Second")
	assert.Len(t, listRunners(t, env), 2)
}

func TestARunnerThatNeverPairedFreesTheSlotOnceItsCodeExpires(t *testing.T) {
	env := browserRunnerEnv(t)
	stale := createRunner(t, env, "Stale")
	_, err := testDB.Exec(context.Background(), `
		UPDATE factory.runners SET pairing_expires_at = now() - interval '1 minute'
		WHERE id = $1`, stale.Runner.Id)
	require.NoError(t, err)

	createRunner(t, env, "Fresh")

	for _, r := range listRunners(t, env) {
		if r.Name == "Stale" {
			assert.NotEmpty(t, r.RevokedAt)
		} else {
			assert.Empty(t, r.RevokedAt)
		}
	}
}

func TestHeartbeatNeedsAValidDeviceToken(t *testing.T) {
	env := browserRunnerEnv(t)
	created := createRunner(t, env, "Home Mac")
	paired := pair(t, env, created.PairingCode)

	for name, ctx := range map[string]context.Context{
		"no header":      context.Background(),
		"wrong scheme":   headerCtx("Authorization", "Token "+paired.DeviceToken),
		"unknown token":  bearer("frt_unknown"),
		"no prefix":      bearer(strings.TrimPrefix(paired.DeviceToken, "frt_")),
		"a session JWT":  bearer("eyJhbGciOi.session.jwt"),
		"the cookie one": headerCtx("Cookie", "accessToken="+paired.DeviceToken),
	} {
		_, err := env.runner.Heartbeat(ctx, &factoryv1.HeartbeatRequest{})
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), name)
	}

	_, err := env.runner.Heartbeat(
		bearer(paired.DeviceToken),
		&factoryv1.HeartbeatRequest{},
	)
	assert.NoError(t, err)
}

func headerCtx(name, value string) context.Context {
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set(name, value)
	return ctx
}

func TestHeartbeatIsHeldOpenForTheConfiguredTime(t *testing.T) {
	const hold = 300 * time.Millisecond
	env := browserRunnerEnv(t, factory.WithHeartbeatHold(hold))
	created := createRunner(t, env, "Home Mac")
	paired := pair(t, env, created.PairingCode)

	start := time.Now()
	_, err := env.runner.Heartbeat(
		bearer(paired.DeviceToken),
		&factoryv1.HeartbeatRequest{},
	)
	took := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, took, hold)
	assert.Less(t, took, 5*hold)
}

func TestHeartbeatStopsWaitingWhenTheRunnerHangsUp(t *testing.T) {
	env := browserRunnerEnv(t, factory.WithHeartbeatHold(time.Minute))
	created := createRunner(t, env, "Home Mac")
	paired := pair(t, env, created.PairingCode)
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.runners SET last_seen = now() - interval '1 hour'`)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(bearer(paired.DeviceToken), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	// The server may answer or the client may time out first: either way the
	// call ends when the runner stops waiting, not after the minute-long hold.
	_, _ = env.runner.Heartbeat(ctx, &factoryv1.HeartbeatRequest{})
	assert.Less(t, time.Since(start), 5*time.Second)

	// The call counted as a sign of life before it was held.
	assert.True(t, listRunners(t, env)[0].Online)
}

func TestRunnerAdminCallsNeedAnAdminInABrowser(t *testing.T) {
	stepUp := factory.WithStepUp(stepUpReturning(nil))
	for name, env := range map[string]runnerEnv{
		"plain user":   newRunnerEnv(t, false, constants.ChannelBrowser, stepUp),
		"bearer admin": newRunnerEnv(t, true, constants.ChannelMCP, stepUp),
	} {
		_, err := env.admin.ListRunners(signedIn(), &factoryv1.ListRunnersRequest{})
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), name)
		_, err = env.admin.CreateRunner(signedIn(),
			&factoryv1.CreateRunnerRequest{Name: "x", TotpCode: "1"})
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), name)
		_, err = env.admin.RevokeRunner(signedIn(),
			&factoryv1.RevokeRunnerRequest{Id: "00000000-0000-0000-0000-000000000000"})
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), name)
	}
}

func TestCreatingARunnerNeedsAPassingStepUp(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want connect.Code
	}{
		"wrong code": {auth.ErrWrongCredential, connect.CodePermissionDenied},
		"locked out": {auth.ErrTooManyAttempts, connect.CodeResourceExhausted},
		"other":      {errors.New("db down"), connect.CodeInternal},
	} {
		env := browserRunnerEnv(t, factory.WithStepUp(stepUpReturning(tc.err)))
		_, err := env.admin.CreateRunner(signedIn(),
			&factoryv1.CreateRunnerRequest{Name: "x", TotpCode: "1"})
		assert.Equal(t, tc.want, connect.CodeOf(err), name)
		assert.Empty(t, listRunners(t, env), name+": nothing was created")
	}
}

func TestCreatingARunnerNeedsASessionAndAConfiguredStepUp(t *testing.T) {
	env := browserRunnerEnv(t)
	_, err := env.admin.CreateRunner(context.Background(),
		&factoryv1.CreateRunnerRequest{Name: "x", TotpCode: "1"})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "no cookie")

	bare := newRunnerEnv(t, true, constants.ChannelBrowser)
	_, err = bare.admin.CreateRunner(signedIn(),
		&factoryv1.CreateRunnerRequest{Name: "x", TotpCode: "1"})
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err), "no step-up")
}

func TestRunnerNamesAndIDsAreValidated(t *testing.T) {
	env := browserRunnerEnv(t)
	for _, name := range []string{"", "  ", strings.Repeat("a", 81), "two\nlines"} {
		_, err := env.admin.CreateRunner(signedIn(),
			&factoryv1.CreateRunnerRequest{Name: name, TotpCode: "1"})
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}

	_, err := env.admin.RevokeRunner(
		signedIn(),
		&factoryv1.RevokeRunnerRequest{Id: "nope"},
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = env.admin.RevokeRunner(signedIn(),
		&factoryv1.RevokeRunnerRequest{Id: "00000000-0000-0000-0000-000000000000"})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestARunnerTokenReachesNoTaskCalls(t *testing.T) {
	env := browserRunnerEnv(t)
	created := createRunner(t, env, "Home Mac")
	paired := pair(t, env, created.PairingCode)
	ts := httptest.NewServer(testhelper.BuildMux(newApp(false, constants.ChannelBrowser)))
	t.Cleanup(ts.Close)
	tasks := factoryv1connect.NewTaskServiceClient(connect.NewClient(
		connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
	))

	// The device token is no session: with it the task API sees no admin.
	_, err := tasks.ListTasks(bearer(paired.DeviceToken), &factoryv1.ListTasksRequest{})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestABadNameIsRefusedBeforeTheStepUpCheck(t *testing.T) {
	env := browserRunnerEnv(
		t,
		factory.WithStepUp(stepUpReturning(auth.ErrWrongCredential)),
	)
	_, err := env.admin.CreateRunner(signedIn(),
		&factoryv1.CreateRunnerRequest{Name: "", TotpCode: "1"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAnEmptyHeartbeatVersionKeepsTheLastOne(t *testing.T) {
	env := browserRunnerEnv(t, factory.WithHeartbeatHold(time.Millisecond))
	created := createRunner(t, env, "Home Mac")
	paired := pair(t, env, created.PairingCode)

	_, err := env.runner.Heartbeat(
		bearer(paired.DeviceToken),
		&factoryv1.HeartbeatRequest{},
	)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", listRunners(t, env)[0].Version)
}

func TestARunnerRevokedDuringAHeartbeatGetsAnError(t *testing.T) {
	env := browserRunnerEnv(t, factory.WithHeartbeatHold(300*time.Millisecond))
	created := createRunner(t, env, "Home Mac")
	paired := pair(t, env, created.PairingCode)

	done := make(chan error, 1)
	go func() {
		_, err := env.runner.Heartbeat(
			bearer(paired.DeviceToken), &factoryv1.HeartbeatRequest{})
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	_, err := env.admin.RevokeRunner(signedIn(),
		&factoryv1.RevokeRunnerRequest{Id: created.Runner.Id})
	require.NoError(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(<-done))
}

package jobs_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/observability/jobs"
)

const metaReleaseFixture = `Dist: noble
Name: Noble Numbat
Version: 24.04 LTS
Date: Thu, 25 Apr 2024 12:00:00 UTC
Supported: 1

Dist: jammy
Name: Jammy Jellyfish
Version: 22.04.4 LTS
Date: Thu, 21 Apr 2022 12:00:00 UTC
Supported: 1
`

const newerMetaReleaseFixture = `Dist: noble
Name: Noble Numbat
Version: 24.04 LTS
Date: Thu, 25 Apr 2024 12:00:00 UTC
Supported: 1

Dist: resolute
Name: Resolute Raccoon
Version: 26.04 LTS
Date: Thu, 23 Apr 2026 00:26:04 UTC
Supported: 0
`

func metaReleaseServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		},
	))
	t.Cleanup(srv.Close)
	return srv
}

func newTestUbuntuReleaseJob(
	t *testing.T, mail *fakeMailer, notified *fakeNotifiedRepo, feedURL string,
) *jobs.UbuntuReleaseJob {
	t.Helper()
	notifSvc := testNotifications(t, mail)
	job := jobs.NewUbuntuReleaseJob(notifSvc, notified)
	jobs.SetUbuntuReleaseFeedURLForTest(job, feedURL)
	return job
}

func TestUbuntuReleaseNoNewReleaseDoesNotEmail(t *testing.T) {
	srv := metaReleaseServer(t, metaReleaseFixture, http.StatusOK)
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()

	job := newTestUbuntuReleaseJob(t, mail, notified, srv.URL)
	require.NoError(t, job.Run(t.Context(), testLogger()))
	testNotifications(t, mail)

	assert.Empty(t, mail.sent)
}

func TestUbuntuReleaseNewReleaseEmailsAndRecordsDedup(t *testing.T) {
	srv := metaReleaseServer(t, newerMetaReleaseFixture, http.StatusOK)
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)
	job := jobs.NewUbuntuReleaseJob(notifSvc, notified)
	jobs.SetUbuntuReleaseFeedURLForTest(job, srv.URL)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.Contains(t, mail.sent[0], "Resolute Raccoon")

	exists, err := notified.Exists(t.Context(), "ubuntu-lts:26.04")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestUbuntuReleaseSkipsAlreadyNotifiedRelease(t *testing.T) {
	srv := metaReleaseServer(t, newerMetaReleaseFixture, http.StatusOK)
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)
	job := jobs.NewUbuntuReleaseJob(notifSvc, notified)
	jobs.SetUbuntuReleaseFeedURLForTest(job, srv.URL)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()
	require.Len(t, mail.sent, 1)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()
	assert.Len(t, mail.sent, 1)
}

func TestUbuntuReleaseHTTPErrorDoesNotFail(t *testing.T) {
	srv := metaReleaseServer(t, "boom", http.StatusInternalServerError)
	mail := &fakeMailer{sent: nil, err: nil}
	notified := newFakeNotifiedRepo()

	job := newTestUbuntuReleaseJob(t, mail, notified, srv.URL)
	require.NoError(t, job.Run(t.Context(), testLogger()))

	assert.Empty(t, mail.sent)
}

func TestUbuntuReleaseMailerNotConfiguredDoesNotRecordAsNotified(t *testing.T) {
	srv := metaReleaseServer(t, newerMetaReleaseFixture, http.StatusOK)
	mail := &fakeMailer{sent: nil, err: fmt.Errorf("not configured")}
	notified := newFakeNotifiedRepo()
	notifSvc := testNotifications(t, mail)
	job := jobs.NewUbuntuReleaseJob(notifSvc, notified)
	jobs.SetUbuntuReleaseFeedURLForTest(job, srv.URL)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	exists, err := notified.Exists(t.Context(), "ubuntu-lts:26.04")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestUbuntuReleaseIDAndRunEvery(t *testing.T) {
	notifSvc := testNotifications(t, &fakeMailer{sent: nil, err: nil})
	job := jobs.NewUbuntuReleaseJob(notifSvc, newFakeNotifiedRepo())

	assert.Equal(t, "notify-ubuntu-lts-release", job.ID())
	assert.Positive(t, job.RunEvery())
}

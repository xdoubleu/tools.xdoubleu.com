package jobs

// SetUbuntuReleaseFeedURLForTest overrides the meta-release feed URL a
// UbuntuReleaseJob fetches from, so tests can point it at an httptest
// server instead of the real Canonical endpoint.
func SetUbuntuReleaseFeedURLForTest(job *UbuntuReleaseJob, url string) {
	job.metaReleaseURL = url
}

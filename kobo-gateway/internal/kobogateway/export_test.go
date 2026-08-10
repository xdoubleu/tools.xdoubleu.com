package kobogateway

import "net/http"

// init swaps launchctl for a no-op under go test — the test runner has no
// real gui/<uid> session, and this keeps `go test` from touching the actual
// login-item state on the machine running it.
//
//nolint:gochecknoinits //only way to stub launchctl before any test runs
func init() {
	launchctl = func(...string) {}
}

// TrustCertArgsForTest exposes trustCertArgs to the _test package.
func TrustCertArgsForTest(certPath string) []string { return trustCertArgs(certPath) }

// NewUpdaterFor builds an Updater with an explicit executable path and
// client, for tests.
func NewUpdaterFor(executable string, client *http.Client) *Updater {
	return &Updater{
		client:         client,
		executablePath: func() (string, error) { return executable, nil },
	}
}

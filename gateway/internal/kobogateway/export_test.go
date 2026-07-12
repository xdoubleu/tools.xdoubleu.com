package kobogateway

// init swaps launchctl for a no-op under go test — the test runner has no
// real gui/<uid> session, and this keeps `go test` from touching the actual
// login-item state on the machine running it.
func init() {
	launchctl = func(...string) {}
}

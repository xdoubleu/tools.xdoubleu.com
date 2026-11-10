package runner

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"tools.xdoubleu.com/selfupdate"
)

const (
	// BinaryName is the artifact the web app serves under /downloads/.
	BinaryName = "factory-runner-darwin-arm64"
	// ReleaseFile names the text file holding the release the served binary was
	// built from.
	ReleaseFile = "factory-runner-release.txt"
	// DevRelease marks a build with no release stamp; it never self-updates.
	DevRelease = "dev"

	// The update runs off the heartbeat path, so a slow link just takes longer.
	downloadTimeout = 5 * time.Minute
)

// ErrUpdateSigningUnset means the build carries no update public key, so no
// download can be trusted.
var ErrUpdateSigningUnset = selfupdate.ErrSigningUnset

// SelfUpdater replaces the running binary with a newer signed release.
type SelfUpdater interface {
	// Update installs the latest release if it differs from the running one, and
	// reports whether it replaced the binary. It re-checks idle right before
	// replacing it, so a job that started during a slow download isn't killed.
	Update(ctx context.Context, idle func() bool) (bool, error)
}

// Updater installs the signed release origin serves over the running binary.
type Updater struct {
	inner   *selfupdate.Updater
	origin  string
	current string
}

// Origin returns scheme://host of the API root: the web app (and its
// downloads) is served at the site root, the API under /api.
func Origin(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid server URL %q", server)
	}
	return u.Scheme + "://" + u.Host, nil
}

// NewUpdater trusts binaries signed by publicKeyB64 (base64 ed25519); an empty
// or invalid key disables self-update. current is the running release.
func NewUpdater(origin, current, publicKeyB64 string) *Updater {
	return &Updater{
		inner: selfupdate.New(
			"runner", BinaryName, ReleaseFile, publicKeyB64, downloadTimeout,
		),
		origin:  origin,
		current: current,
	}
}

// Update installs the latest release when it differs from the running one.
func (u *Updater) Update(ctx context.Context, idle func() bool) (bool, error) {
	if u.current == DevRelease || u.current == "" {
		return false, nil
	}
	if u.inner.PublicKey == nil {
		return false, ErrUpdateSigningUnset
	}
	latest, err := u.inner.Release(ctx, u.origin)
	if err != nil {
		return false, err
	}
	if latest == "" || latest == u.current {
		return false, nil
	}
	return u.inner.Install(ctx, u.origin, idle)
}

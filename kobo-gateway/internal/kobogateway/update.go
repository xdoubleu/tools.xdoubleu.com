package kobogateway

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"tools.xdoubleu.com/selfupdate"
)

const (
	// BinaryName is the artifact the web app serves under /downloads/.
	BinaryName = "kobo-gateway-darwin-arm64"

	// ReleaseFile names the text file holding the release BinaryName was built
	// from.
	ReleaseFile = "kobo-gateway-release.txt"

	downloadTimeout = 60 * time.Second
)

// ErrUpdateSigningUnset means the build carries no update public key, so no
// download can be trusted.
var ErrUpdateSigningUnset = selfupdate.ErrSigningUnset

// Updater implements UpdateRunner by atomically replacing the running
// executable with a download signed by the release key.
type Updater struct {
	inner *selfupdate.Updater
}

// NewUpdater builds an Updater that replaces the current executable, trusting
// binaries signed by publicKeyB64 (base64 ed25519). An empty or invalid key
// disables self-update.
func NewUpdater(publicKeyB64 string) *Updater {
	return &Updater{inner: selfupdate.New(
		"gateway", BinaryName, ReleaseFile, publicKeyB64, downloadTimeout,
	)}
}

// LatestRelease returns the release origin currently serves.
func (u *Updater) LatestRelease(ctx context.Context, origin string) (string, error) {
	return u.inner.Release(ctx, origin)
}

// SelfUpdate installs origin's gateway binary over the current executable and
// re-signs its app bundle. On any failure the running binary is left untouched.
func (u *Updater) SelfUpdate(ctx context.Context, origin string) error {
	if _, err := u.inner.Install(ctx, origin, nil); err != nil {
		return err
	}
	executable, err := u.inner.ExecutablePath()
	if err != nil {
		return fmt.Errorf("could not resolve executable path: %w", err)
	}
	if err = resignBundle(ctx, executable); err != nil {
		return fmt.Errorf("could not re-sign updated app bundle: %w", err)
	}
	return nil
}

// resignBundle re-applies package.sh's ad-hoc signature, which covers the
// binary just overwritten. A raw dev binary has nothing to re-sign.
func resignBundle(ctx context.Context, executable string) error {
	appDir := AppBundlePath(executable)
	if appDir == "" {
		return nil
	}

	return exec.CommandContext(ctx, "codesign", "--force", "--sign", "-", appDir).Run()
}

// AppBundlePath returns the .app bundle directory containing executable, or
// "" for a raw dev binary.
func AppBundlePath(executable string) string {
	appDir := filepath.Dir(filepath.Dir(filepath.Dir(executable))) // .../KoboGateway.app
	if filepath.Ext(appDir) != ".app" {
		return ""
	}

	return appDir
}

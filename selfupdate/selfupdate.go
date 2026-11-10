// Package selfupdate replaces a running macOS binary with a newer one served
// under /downloads/ and signed with the release key (see package updatesig).
package selfupdate

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"tools.xdoubleu.com/selfupdate/updatesig"
)

const (
	downloadsPath    = "/downloads/"
	maxBinarySize    = 512 << 20
	maxSmallDownload = 1 << 10
	binaryPerm       = 0o755
)

// ErrSigningUnset means the build carries no update public key, so no download
// can be trusted.
var ErrSigningUnset = errors.New("self-update is disabled: no update signing key")

// Updater downloads origin/downloads/<BinaryName>, checks its signature against
// PublicKey and atomically renames it over the running executable. Any failure
// leaves the running binary untouched. Go's HTTP client sets no quarantine
// attribute, so Gatekeeper isn't re-triggered.
type Updater struct {
	Client *http.Client
	// ExecutablePath is os.Executable, injectable for tests.
	ExecutablePath func() (string, error)
	// PublicKey verifies downloads; nil disables self-update.
	PublicKey ed25519.PublicKey
	// BinaryName and ReleaseFile are the served artifact and the text file
	// naming the release it was built from.
	BinaryName  string
	ReleaseFile string
	// Kind names the binary in errors, e.g. "gateway".
	Kind string
}

// New trusts binaries signed by publicKeyB64 (base64 ed25519); an empty or
// invalid key disables self-update.
func New(
	kind, binaryName, releaseFile, publicKeyB64 string,
	timeout time.Duration,
) *Updater {
	publicKey, err := updatesig.ParsePublicKey(publicKeyB64)
	if err != nil {
		publicKey = nil
	}
	return &Updater{
		Client:         &http.Client{Timeout: timeout},
		ExecutablePath: os.Executable,
		PublicKey:      publicKey,
		BinaryName:     binaryName,
		ReleaseFile:    releaseFile,
		Kind:           kind,
	}
}

// Release returns the release origin currently serves.
func (u *Updater) Release(ctx context.Context, origin string) (string, error) {
	raw, err := u.download(ctx, origin+downloadsPath+u.ReleaseFile, maxSmallDownload)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

// Install replaces the running executable with origin's binary. idle is asked
// right before the swap (nil means always); false leaves the binary alone and
// reports replaced=false.
func (u *Updater) Install(
	ctx context.Context,
	origin string,
	idle func() bool,
) (bool, error) {
	if u.PublicKey == nil {
		return false, ErrSigningUnset
	}
	executable, err := u.ExecutablePath()
	if err != nil {
		return false, fmt.Errorf("could not resolve executable path: %w", err)
	}
	binaryURL := origin + downloadsPath + u.BinaryName
	data, err := u.download(ctx, binaryURL, maxBinarySize)
	if err != nil {
		return false, err
	}
	sig, err := u.download(ctx, binaryURL+".sig", maxSmallDownload)
	if err != nil {
		return false, err
	}
	if err = updatesig.Verify(u.PublicKey, data, string(sig)); err != nil {
		return false, err
	}
	if !isMachO(data) {
		return false, fmt.Errorf("downloaded file is not a valid %s binary", u.Kind)
	}
	if idle != nil && !idle() {
		return false, nil
	}

	tmp := executable + ".update"
	if err = os.WriteFile(tmp, data, binaryPerm); err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("could not write updated binary: %w", err)
	}
	if err = os.Rename(tmp, executable); err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("could not replace running binary: %w", err)
	}
	return true, nil
}

func (u *Updater) download(
	ctx context.Context,
	downloadURL string,
	maxSize int64,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build download request: %w", err)
	}
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update download failed: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return nil, fmt.Errorf("could not read update: %w", err)
	}
	return data, nil
}

// isMachO accepts 64-bit little-endian Mach-O (darwin/arm64) and universal
// binaries.
func isMachO(data []byte) bool {
	machO64 := []byte{0xcf, 0xfa, 0xed, 0xfe}
	fat := []byte{0xca, 0xfe, 0xba, 0xbe}
	return bytes.HasPrefix(data, machO64) || bytes.HasPrefix(data, fat)
}

package runner

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"tools.xdoubleu.com/factory-runner/internal/updatesig"
)

const (
	// BinaryName is the artifact the web app serves under /downloads/.
	BinaryName = "factory-runner-darwin-arm64"
	// ReleaseFile names the text file holding the release the served binary was
	// built from.
	ReleaseFile = "factory-runner-release.txt"
	// DevRelease marks a build with no release stamp; it never self-updates.
	DevRelease = "dev"

	downloadsPath    = "/downloads/"
	downloadTimeout  = 60 * time.Second
	maxBinarySize    = 512 << 20
	maxSmallDownload = 1 << 10
	binaryPerm       = 0o755
)

// ErrUpdateSigningUnset means the build carries no update public key, so no
// download can be trusted.
var ErrUpdateSigningUnset = errors.New(
	"self-update is disabled: no update signing key",
)

// SelfUpdater replaces the running binary with a newer signed release.
type SelfUpdater interface {
	// Update installs the latest release if it differs from the running one, and
	// reports whether it replaced the binary.
	Update(ctx context.Context) (bool, error)
}

// Updater downloads origin/downloads/<BinaryName>, checks its ed25519 signature
// against the key compiled into the build, and atomically renames it over the
// running executable. Any failure leaves the running binary untouched. Go's
// HTTP client sets no quarantine attribute, so Gatekeeper isn't re-triggered.
type Updater struct {
	client         *http.Client
	executablePath func() (string, error)
	publicKey      ed25519.PublicKey
	origin         string
	current        string
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
	publicKey, err := updatesig.ParsePublicKey(publicKeyB64)
	if err != nil {
		publicKey = nil
	}
	return &Updater{
		client:         &http.Client{Timeout: downloadTimeout},
		executablePath: os.Executable,
		publicKey:      publicKey,
		origin:         origin,
		current:        current,
	}
}

// Update installs the latest release when it differs from the running one.
func (u *Updater) Update(ctx context.Context) (bool, error) {
	if u.publicKey == nil {
		return false, ErrUpdateSigningUnset
	}
	if u.current == DevRelease || u.current == "" {
		return false, nil
	}

	raw, err := u.download(ctx, u.origin+downloadsPath+ReleaseFile, maxSmallDownload)
	if err != nil {
		return false, err
	}
	latest := strings.TrimSpace(string(raw))
	if latest == "" || latest == u.current {
		return false, nil
	}

	executable, err := u.executablePath()
	if err != nil {
		return false, fmt.Errorf("resolve executable path: %w", err)
	}
	data, err := u.download(ctx, u.origin+downloadsPath+BinaryName, maxBinarySize)
	if err != nil {
		return false, err
	}
	sig, err := u.download(
		ctx, u.origin+downloadsPath+BinaryName+".sig", maxSmallDownload,
	)
	if err != nil {
		return false, err
	}
	if err = updatesig.Verify(u.publicKey, data, string(sig)); err != nil {
		return false, err
	}
	if !isMachO(data) {
		return false, errors.New("downloaded file is not a valid runner binary")
	}

	tmp := executable + ".update"
	if err = os.WriteFile(tmp, data, binaryPerm); err != nil {
		return false, fmt.Errorf("write updated binary: %w", err)
	}
	if err = os.Rename(tmp, executable); err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("replace running binary: %w", err)
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
		return nil, fmt.Errorf("build download request: %w", err)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update download failed: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return nil, fmt.Errorf("read update: %w", err)
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

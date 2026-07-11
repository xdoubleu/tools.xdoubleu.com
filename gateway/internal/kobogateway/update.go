package kobogateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// BinaryName is the artifact the web app serves under /downloads/.
	BinaryName = "kobo-gateway-darwin-arm64"

	// DownloadPath is where the web app serves the latest gateway binary.
	DownloadPath = "/downloads/" + BinaryName

	downloadTimeout = 60 * time.Second

	// maxBinarySize caps the self-update download (the real binary is a
	// few MB) so a misbehaving server cannot exhaust memory or disk.
	maxBinarySize = 512 << 20
)

// Updater implements UpdateRunner by downloading the latest binary from a
// trusted origin and atomically replacing the running executable. The
// download uses Go's HTTP client, so the new file carries no
// com.apple.quarantine attribute and Gatekeeper is not re-triggered.
type Updater struct {
	client *http.Client
	// executablePath is os.Executable, injectable for tests.
	executablePath func() (string, error)
}

// NewUpdater builds an Updater that replaces the current executable.
func NewUpdater() *Updater {
	return &Updater{
		client:         &http.Client{Timeout: downloadTimeout},
		executablePath: os.Executable,
	}
}

// NewUpdaterFor builds an Updater with an explicit executable path and
// client, for tests.
func NewUpdaterFor(executable string, client *http.Client) *Updater {
	return &Updater{
		client:         client,
		executablePath: func() (string, error) { return executable, nil },
	}
}

// SelfUpdate downloads origin+DownloadPath to a temp file next to the
// current executable, sanity-checks it, and atomically renames it over the
// executable. On any failure the running binary is left untouched.
func (u *Updater) SelfUpdate(ctx context.Context, origin string) error {
	executable, err := u.executablePath()
	if err != nil {
		return fmt.Errorf("could not resolve executable path: %w", err)
	}

	data, err := u.download(ctx, origin+DownloadPath)
	if err != nil {
		return err
	}

	if !isMachO(data) {
		return errors.New("downloaded file is not a valid gateway binary")
	}

	tmpPath := executable + ".update"
	//nolint:gosec //the downloaded gateway binary must be executable
	if err = os.WriteFile(tmpPath, data, 0o755); err != nil {
		return fmt.Errorf("could not write updated binary: %w", err)
	}

	if err = os.Rename(tmpPath, executable); err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("could not replace running binary: %w", err)
	}

	return nil
}

func (u *Updater) download(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		downloadURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not build download request: %w", err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update download failed: %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBinarySize))
	if err != nil {
		return nil, fmt.Errorf("could not read update: %w", err)
	}

	return data, nil
}

// isMachO accepts 64-bit Mach-O binaries (little endian, as produced for
// darwin/arm64) and universal fat binaries.
func isMachO(data []byte) bool {
	machO64 := []byte{0xcf, 0xfa, 0xed, 0xfe}
	fat := []byte{0xca, 0xfe, 0xba, 0xbe}

	return bytes.HasPrefix(data, machO64) || bytes.HasPrefix(data, fat)
}

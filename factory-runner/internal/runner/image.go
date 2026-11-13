package runner

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"tools.xdoubleu.com/selfupdate/updatesig"
)

// The name and size limits of the job-image artifacts served under /downloads/
// (built and signed by build-factory-runner.yml). The tarball is the OCI archive
// `container image load` reads; the manifest names the release and its sha256.
const (
	JobImageManifest = "factory-job-image.txt"
	JobImageArchive  = "factory-job-image.tar"

	maxManifestBytes = 1 << 10
	maxImageBytes    = 8 << 30 // 8 GiB: the baked toolchains and caches are large.
	imageFilePerm    = 0o600

	// ImageFetchTimeout bounds a single download via the release server.
	ImageFetchTimeout = 30 * time.Minute
)

// ImageEnsurer makes sure the versioned job image a release needs is present in
// the local `container` image store, fetching and loading it from the release
// server if it is not. It runs at job-runner start, so also after a self-update.
// A run that cannot verify or load the image fails loudly rather than leaving a
// job to run on a stale or missing one.
type ImageEnsurer struct {
	// Client downloads from Origin/downloads/.
	Client *http.Client
	// Origin is scheme://host of the release server (the web app's root).
	Origin string
	// Release is the running binary's release; the manifest must name it.
	Release string
	// Image is the exact reference to ensure is present (factory-job:<release>).
	Image string
	// PublicKey verifies the manifest's signature; nil disables the fetch, as
	// with self-update.
	PublicKey ed25519.PublicKey
	// Exec runs `container` on the host.
	Exec Execer
	// TempDir stages the tarball; empty means os.TempDir().
	TempDir string
}

// Present reports whether Image is already in the local image store.
func (e *ImageEnsurer) Present(ctx context.Context) (bool, error) {
	code, err := e.Exec.Exec(
		ctx, []string{containerCLI, "image", "inspect", e.Image}, nil, io.Discard,
	)
	if err != nil {
		return false, err
	}
	return code == 0, nil
}

// Ensure returns once Image is present, fetching and loading it if needed. The
// manifest's signature and release are verified before the archive is trusted,
// and its sha256 is checked as it streams to disk.
func (e *ImageEnsurer) Ensure(ctx context.Context) error {
	if e.PublicKey == nil {
		return fmt.Errorf("no update signing key to verify a job image")
	}
	if present, err := e.Present(ctx); err != nil {
		return err
	} else if present {
		return nil
	}
	downloads := e.Origin + "/downloads/"
	manifest, err := e.download(ctx, downloads+JobImageManifest, maxManifestBytes)
	if err != nil {
		return err
	}
	sig, err := e.download(ctx, downloads+JobImageManifest+".sig", maxManifestBytes)
	if err != nil {
		return err
	}
	if err = updatesig.Verify(e.PublicKey, manifest, string(sig)); err != nil {
		return fmt.Errorf("job image manifest: %w", err)
	}
	release, sumHex, ok := strings.Cut(strings.TrimSpace(string(manifest)), " ")
	if !ok || release != e.Release {
		return fmt.Errorf("job image manifest is for %q, not release %q",
			release, e.Release)
	}
	sum := strings.TrimSpace(sumHex)
	if decoded, derr := hex.DecodeString(sum); derr != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("job image manifest has a bad checksum %q", sum)
	}
	path, err := e.downloadVerified(ctx, downloads+JobImageArchive, sum)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	code, err := e.Exec.Exec(
		ctx, []string{containerCLI, "image", "load"}, file, io.Discard,
	)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("container image load failed (exit %d)", code)
	}
	present, err := e.Present(ctx)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("the loaded image did not appear as %s", e.Image)
	}
	return nil
}

// download fetches a small signed file wholly into memory.
func (e *ImageEnsurer) download(
	ctx context.Context, url string, maxSize int64,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxSize))
}

// downloadVerified streams a large archive to a temp file, checking its sha256 as
// it is written, and returns the path.
func (e *ImageEnsurer) downloadVerified(
	ctx context.Context, url, wantSum string,
) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := e.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", url, resp.Status)
	}
	dir := e.TempDir
	if dir == "" {
		dir = os.TempDir()
	}
	file, err := os.CreateTemp(dir, "job-image-*.tar")
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	// The caller holds the file; a failed write is removed, never loaded.
	if _, copyErr := io.Copy(
		io.MultiWriter(file, hash), io.LimitReader(resp.Body, maxImageBytes),
	); copyErr != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("download %s: %w", url, copyErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		_ = os.Remove(file.Name())
		return "", closeErr
	}
	if res := hex.EncodeToString(hash.Sum(nil)); res != wantSum {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("job image sha256 %s does not match %s", res, wantSum)
	}
	if chmodErr := os.Chmod(file.Name(), imageFilePerm); chmodErr != nil {
		_ = os.Remove(file.Name())
		return "", chmodErr
	}
	return file.Name(), nil
}

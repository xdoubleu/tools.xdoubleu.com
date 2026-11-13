package runner

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tools.xdoubleu.com/selfupdate/updatesig"
)

// JobImageManifest is the signed per-commit manifest (build-factory-runner.yml)
// served under /downloads/: "<release> <image digest hex>". The image itself
// lives on GHCR and is pulled by that digest.
const (
	JobImageManifest = "factory-job-image.txt"
	// JobImageRepo is the public GHCR repository holding the job image.
	JobImageRepo = "ghcr.io/xdoubleu/tools.xdoubleu.com/factory-job-image"

	maxManifestBytes = 1 << 10
	digestHexLen     = 64

	// ImageFetchTimeout bounds a single manifest download.
	ImageFetchTimeout = 30 * time.Second
)

// ImageEnsurer makes sure the versioned job image a release needs is present in
// the local `container` image store, pulling it from GHCR by the digest in the
// signed manifest if it is not. It runs at job-runner start, so also after a
// self-update. A run that cannot verify or pull the image fails loudly rather
// than leaving a job to run on a stale or missing one.
type ImageEnsurer struct {
	// Client downloads the manifest from Origin/downloads/.
	Client *http.Client
	// Origin is scheme://host of the release server (the web app's root).
	Origin string
	// Release is the running binary's release; the manifest must name it.
	Release string
	// Image is the local reference to ensure is present (factory-job:<release>).
	Image string
	// PublicKey verifies the manifest's signature; nil disables the fetch, as
	// with self-update.
	PublicKey ed25519.PublicKey
	// Exec runs `container` on the host.
	Exec Execer
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

// Ensure returns once Image is present, pulling it if needed. The manifest's
// signature and release are verified before its digest is trusted; the pulled
// image must report that digest and is then tagged as Image.
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
	release, digest, ok := strings.Cut(strings.TrimSpace(string(manifest)), " ")
	if !ok || release != e.Release {
		return fmt.Errorf("job image manifest is for %q, not release %q",
			release, e.Release)
	}
	digest = strings.TrimSpace(digest)
	if decoded, derr := hex.DecodeString(digest); derr != nil ||
		len(digest) != digestHexLen || len(decoded)*2 != digestHexLen {
		return fmt.Errorf("job image manifest has a bad digest %q", digest)
	}
	ref := JobImageRepo + "@sha256:" + digest
	if err = e.run(ctx, nil, "image", "pull", ref); err != nil {
		return err
	}
	var inspected bytes.Buffer
	if err = e.run(ctx, &inspected, "image", "inspect", ref); err != nil {
		return err
	}
	if !strings.Contains(inspected.String(), digest) {
		return fmt.Errorf("pulled job image does not report digest sha256:%s", digest)
	}
	if err = e.run(ctx, nil, "image", "tag", ref, e.Image); err != nil {
		return err
	}
	present, err := e.Present(ctx)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("the pulled image did not appear as %s", e.Image)
	}
	return nil
}

// run executes a `container` subcommand, failing on a non-zero exit.
func (e *ImageEnsurer) run(
	ctx context.Context, out *bytes.Buffer, args ...string,
) error {
	sink := io.Discard
	if out != nil {
		sink = out
	}
	code, err := e.Exec.Exec(ctx, append([]string{containerCLI}, args...), nil, sink)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("container %s failed (exit %d)", args[0]+" "+args[1], code)
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

package runner_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/selfupdate/updatesig"
)

// imageServer signs and serves a job-image manifest and archive, counting hits.
type imageServer struct {
	manifest []byte
	archive  []byte
	seedB64  string
	hits     map[string]int
	mu       sync.Mutex
}

func (s *imageServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.hits[r.URL.Path]++
	s.mu.Unlock()
	switch r.URL.Path {
	case "/downloads/" + runner.JobImageManifest:
		_, _ = w.Write(s.manifest)
	case "/downloads/" + runner.JobImageManifest + ".sig":
		sig, err := updatesig.Sign(s.seedB64, s.manifest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, sig)
	case "/downloads/" + runner.JobImageArchive:
		_, _ = w.Write(s.archive)
	default:
		http.NotFound(w, r)
	}
}

func (s *imageServer) count(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[path]
}

// newImageServer returns a server plus the public key matching the seed it signs
// with, and a manifest for the release holding the archive's real sha256.
func newImageServer(release string, archive []byte) (*imageServer, string) {
	sum := sha256.Sum256(archive)
	return newImageServerManifest(
		fmt.Sprintf("%s %s\n", release, hex.EncodeToString(sum[:])), archive,
	)
}

// newImageServerManifest serves an arbitrary manifest and archive, and returns a
// public key matching the seed that signs the manifest.
func newImageServerManifest(manifest string, archive []byte) (*imageServer, string) {
	pubB64, seedB64, err := updatesig.GenerateKey()
	if err != nil {
		panic(err)
	}
	return &imageServer{
		manifest: []byte(manifest),
		archive:  archive,
		hits:     map[string]int{},
		mu:       sync.Mutex{},
		seedB64:  seedB64,
	}, pubB64
}

// imageAnswerScript scripts `container image inspect` (absent the first
// absentChecks times, present after) and counts `container image load` calls.
func imageAnswerScript(absentChecks int, onLoad func()) func(
	[]string,
) (int, string, error) {
	var inspects, loads int
	return func(argv []string) (int, string, error) {
		if len(argv) >= 3 && argv[0] == "container" && argv[1] == "image" {
			switch argv[2] {
			case "inspect":
				inspects++
				if inspects > absentChecks {
					return 0, "", nil
				}
				return 1, "no such image", nil
			case "load":
				loads++
				if onLoad != nil {
					onLoad()
				}
				return 0, "", nil
			}
		}
		return 0, "", nil
	}
}

func newImageEnsurer(
	t *testing.T,
	srv *httptest.Server,
	pubB64 string,
	answer func(argv []string) (int, string, error),
) *runner.ImageEnsurer {
	t.Helper()
	publicKey, err := updatesig.ParsePublicKey(pubB64)
	require.NoError(t, err)
	return &runner.ImageEnsurer{
		Client:    srv.Client(),
		Origin:    srv.URL,
		Release:   "abc123",
		Image:     "factory-job:abc123",
		PublicKey: publicKey,
		Exec:      newExecScript(answer),
		TempDir:   t.TempDir(),
	}
}

func TestEnsureImageArchivePublicUnavailable(t *testing.T) {
	// Manifest and signature serve, but the archive 404s.
	archive := []byte("x")
	srv, pubB64 := newImageServer("abc123", archive)
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/downloads/"+runner.JobImageArchive {
				http.NotFound(w, r)
				return
			}
			srv.ServeHTTP(w, r)
		}))
	defer ts.Close()

	e := newImageEnsurer(t, ts, pubB64, imageAnswerScript(1, nil))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download")
}

func TestEnsureImageCannotStageTheArchive(t *testing.T) {
	archive := []byte("x")
	srv, pubB64 := newImageServer("abc123", archive)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	e := newImageEnsurer(t, ts, pubB64, imageAnswerScript(1, nil))
	e.TempDir = filepath.Join(t.TempDir(), "does", "not", "exist")
	err := e.Ensure(context.Background())
	require.Error(t, err)
}

func TestEnsureImageAlreadyPresent(t *testing.T) {
	srv, pubB64 := newImageServer("abc123", []byte("archive"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	e := newImageEnsurer(t, ts, pubB64,
		imageAnswerScript(0, nil))
	require.NoError(t, e.Ensure(context.Background()))
	assert.Zero(t, srv.count("/downloads/"+runner.JobImageManifest),
		"an already-present image is not fetched")
}

func TestEnsureImageFetchesAndLoads(t *testing.T) {
	srv, pubB64 := newImageServer("abc123", []byte("the oci archive"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var loads int
	e := newImageEnsurer(t, ts, pubB64,
		imageAnswerScript(1, func() { loads++ }))
	require.NoError(t, e.Ensure(context.Background()))
	assert.Equal(t, 1, loads, "the archive is loaded once")
	assert.Equal(t, 1, srv.count("/downloads/"+runner.JobImageManifest))
	assert.Equal(t, 1, srv.count("/downloads/"+runner.JobImageArchive))
}

func TestEnsureImageRejectsWrongRelease(t *testing.T) {
	srv, pubB64 := newImageServer("def456", []byte("x"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	e := newImageEnsurer(t, ts, pubB64,
		imageAnswerScript(1, nil))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not release")
}

func TestEnsureImageRejectsBadSignature(t *testing.T) {
	srv, _ := newImageServer("abc123", []byte("x"))
	// A public key that is not the server's signing key fails verification.
	_, other, _ := updatesig.GenerateKey() // pubB64, seedB64
	ts := httptest.NewServer(srv)
	defer ts.Close()
	e := newImageEnsurer(t, ts, other,
		imageAnswerScript(1, nil))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest")
}

func TestEnsureImageRejectsChecksumMismatch(t *testing.T) {
	// The manifest promises a sha256 that the served archive does not have.
	srv, pubB64 := newImageServerManifest(
		"abc123 "+strings.Repeat("0", 64)+"\n", []byte("the real archive"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	e := newImageEnsurer(t, ts, pubB64,
		imageAnswerScript(1, nil))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestEnsureImageRequiresSigningKey(t *testing.T) {
	e := &runner.ImageEnsurer{
		Client:    &http.Client{},
		Origin:    "https://example.com",
		Release:   "abc123",
		Image:     "factory-job:abc123",
		PublicKey: nil,
		Exec:      newExecScript(nil),
		TempDir:   "",
	}
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signing key")
}

func TestEnsureImagePresentFails(t *testing.T) {
	srv, pubB64 := newImageServer("abc123", []byte("x"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// The container CLI is unusable; Present reports the failure.
	answer := func([]string) (int, string, error) {
		return -1, "container: not found", errors.New("container: not found")
	}
	e := newImageEnsurer(t, ts, pubB64, answer)
	present, err := e.Present(context.Background())
	require.Error(t, err)
	assert.False(t, present)
}

func TestEnsureImageLoadFails(t *testing.T) {
	archive := []byte("x")
	srv, pubB64 := newImageServer("abc123", archive)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	answer := func(argv []string) (int, string, error) {
		if len(argv) >= 3 && argv[0] == "container" && argv[1] == "image" &&
			argv[2] == "load" {
			return 1, "load refused", nil
		}
		return imageAnswerScript(1, nil)(argv)
	}
	e := newImageEnsurer(t, ts, pubB64, answer)
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image load failed")
}

func TestEnsureImageManifestUnavailable(t *testing.T) {
	// Nothing is served, so the first download fails with a clear error.
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()
	pubB64, _, _ := updatesig.GenerateKey()
	e := newImageEnsurer(t, ts, pubB64, imageAnswerScript(1, nil))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download")
}

func TestEnsureImageArchiveChecksumStreamed(t *testing.T) {
	// A larger archive exercises the whole streaming/checksum path end to end.
	archive := make([]byte, 1<<20)
	for i := range archive {
		archive[i] = byte(i)
	}
	srv, pubB64 := newImageServer("abc123", archive)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	e := newImageEnsurer(t, ts, pubB64, imageAnswerScript(1, nil))
	require.NoError(t, e.Ensure(context.Background()))
}

func TestEnsureImageErrorsWhenLoadNotPresent(t *testing.T) {
	srv, pubB64 := newImageServer("abc123", []byte("x"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// The archive loads but never appears as the expected tag.
	e := newImageEnsurer(t, ts, pubB64,
		imageAnswerScript(2, nil))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not appear")
}

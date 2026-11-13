package runner_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/selfupdate/updatesig"
)

const testDigest = "abababababababababababababababababababababababababababababababab"

// imageServer signs and serves a job-image manifest, counting hits.
type imageServer struct {
	manifest []byte
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
		_, _ = w.Write([]byte(sig))
	default:
		http.NotFound(w, r)
	}
}

func (s *imageServer) count(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[path]
}

// newImageServer serves a signed manifest and returns the matching public key.
func newImageServer(manifest string) (*imageServer, string) {
	pubB64, seedB64, err := updatesig.GenerateKey()
	if err != nil {
		panic(err)
	}
	return &imageServer{
		manifest: []byte(manifest),
		hits:     map[string]int{},
		mu:       sync.Mutex{},
		seedB64:  seedB64,
	}, pubB64
}

// imageAnswerScript scripts `container image inspect`: the local tag is absent
// the first absentChecks times; the digest ref reports inspectOut. Pull and tag
// calls are counted.
func imageAnswerScript(absentChecks int, inspectOut string, pulls, tags *[]string) func(
	[]string,
) (int, string, error) {
	var inspects int
	return func(argv []string) (int, string, error) {
		if len(argv) >= 4 && argv[0] == "container" && argv[1] == "image" {
			switch argv[2] {
			case "inspect":
				if strings.Contains(argv[3], "@sha256:") {
					return 0, inspectOut, nil
				}
				inspects++
				if inspects > absentChecks {
					return 0, "", nil
				}
				return 1, "no such image", nil
			case "pull":
				*pulls = append(*pulls, argv[3])
			case "tag":
				*tags = append(*tags, argv[4])
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
	}
}

func TestEnsureImagePullsByDigestAndTags(t *testing.T) {
	srv, pubB64 := newImageServer("abc123 " + testDigest + "\n")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var pulls, tags []string
	e := newImageEnsurer(t, ts, pubB64,
		imageAnswerScript(1, `[{"digest":"sha256:`+testDigest+`"}]`, &pulls, &tags))
	require.NoError(t, e.Ensure(context.Background()))
	assert.Equal(t, []string{runner.JobImageRepo + "@sha256:" + testDigest}, pulls)
	assert.Equal(t, []string{"factory-job:abc123"}, tags)
}

func TestEnsureImageAlreadyPresent(t *testing.T) {
	srv, pubB64 := newImageServer("abc123 " + testDigest + "\n")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var pulls, tags []string
	e := newImageEnsurer(t, ts, pubB64, imageAnswerScript(0, "", &pulls, &tags))
	require.NoError(t, e.Ensure(context.Background()))
	assert.Zero(t, srv.count("/downloads/"+runner.JobImageManifest),
		"an already-present image is not fetched")
	assert.Empty(t, pulls)
}

func TestEnsureImageRejectsWrongRelease(t *testing.T) {
	srv, pubB64 := newImageServer("def456 " + testDigest + "\n")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var pulls, tags []string
	e := newImageEnsurer(t, ts, pubB64, imageAnswerScript(1, "", &pulls, &tags))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not release")
	assert.Empty(t, pulls)
}

func TestEnsureImageRejectsBadSignature(t *testing.T) {
	srv, _ := newImageServer("abc123 " + testDigest + "\n")
	_, other, _ := updatesig.GenerateKey()
	ts := httptest.NewServer(srv)
	defer ts.Close()
	var pulls, tags []string
	e := newImageEnsurer(t, ts, other, imageAnswerScript(1, "", &pulls, &tags))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest")
	assert.Empty(t, pulls)
}

func TestEnsureImageRejectsBadDigest(t *testing.T) {
	srv, pubB64 := newImageServer("abc123 nothex\n")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var pulls, tags []string
	e := newImageEnsurer(t, ts, pubB64, imageAnswerScript(1, "", &pulls, &tags))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad digest")
}

func TestEnsureImageRejectsPulledDigestMismatch(t *testing.T) {
	srv, pubB64 := newImageServer("abc123 " + testDigest + "\n")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var pulls, tags []string
	e := newImageEnsurer(t, ts, pubB64,
		imageAnswerScript(1, `[{"digest":"sha256:other"}]`, &pulls, &tags))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not report digest")
	assert.Empty(t, tags)
}

func TestEnsureImageRequiresSigningKey(t *testing.T) {
	e := &runner.ImageEnsurer{
		Client:    &http.Client{},
		Origin:    "https://example.com",
		Release:   "abc123",
		Image:     "factory-job:abc123",
		PublicKey: nil,
		Exec:      newExecScript(nil),
	}
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signing key")
}

func TestEnsureImagePresentFails(t *testing.T) {
	srv, pubB64 := newImageServer("abc123 " + testDigest + "\n")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	answer := func([]string) (int, string, error) {
		return -1, "container: not found", errors.New("container: not found")
	}
	e := newImageEnsurer(t, ts, pubB64, answer)
	present, err := e.Present(context.Background())
	require.Error(t, err)
	assert.False(t, present)
}

func TestEnsureImagePullFails(t *testing.T) {
	srv, pubB64 := newImageServer("abc123 " + testDigest + "\n")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var pulls, tags []string
	base := imageAnswerScript(1, "", &pulls, &tags)
	answer := func(argv []string) (int, string, error) {
		if len(argv) >= 3 && argv[2] == "pull" {
			return 1, "pull refused", nil
		}
		return base(argv)
	}
	e := newImageEnsurer(t, ts, pubB64, answer)
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image pull failed")
}

func TestEnsureImageManifestUnavailable(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()
	pubB64, _, _ := updatesig.GenerateKey()
	var pulls, tags []string
	e := newImageEnsurer(t, ts, pubB64, imageAnswerScript(1, "", &pulls, &tags))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download")
}

func TestEnsureImageErrorsWhenTagNotPresent(t *testing.T) {
	srv, pubB64 := newImageServer("abc123 " + testDigest + "\n")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var pulls, tags []string
	e := newImageEnsurer(t, ts, pubB64,
		imageAnswerScript(2, "sha256:"+testDigest, &pulls, &tags))
	err := e.Ensure(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not appear")
}

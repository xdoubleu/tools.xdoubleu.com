package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/selfupdate/updatesig"
)

// imageEnsureExec scripts the container CLI the ImageEnsurer drives: inspect
// reports the image absent then present and echoes the digest for a digest ref.
type imageEnsureExec struct{ inspects int }

func (e *imageEnsureExec) Exec(
	_ context.Context,
	argv []string,
	_ io.Reader,
	out io.Writer,
) (int, error) {
	if len(argv) >= 4 && argv[0] == "container" && argv[1] == "image" &&
		argv[2] == "inspect" {
		if strings.Contains(argv[3], "@sha256:") {
			_, _ = io.WriteString(out, argv[3])
			return 0, nil
		}
		e.inspects++
		if e.inspects > 1 {
			return 0, nil
		}
		return 1, nil
	}
	return 0, nil
}

// imageEnsureIndex serves the signed manifest and its signature.
type imageEnsureIndex struct {
	manifest []byte
	seedB64  string
}

func (s *imageEnsureIndex) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/downloads/" + runner.JobImageManifest:
		_, _ = w.Write(s.manifest)
	case "/downloads/" + runner.JobImageManifest + ".sig":
		sig, _ := updatesig.Sign(s.seedB64, s.manifest)
		_, _ = io.WriteString(w, sig)
	default:
		http.NotFound(w, r)
	}
}

func ensureServer(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	pubB64, seedB64, err := updatesig.GenerateKey()
	require.NoError(t, err)
	manifest := []byte(fmt.Sprintf("abc123 %s\n", strings.Repeat("cd", 32)))
	ts := httptest.NewServer(&imageEnsureIndex{
		manifest: manifest, seedB64: seedB64,
	})
	return ts, pubB64, seedB64
}

func testEnsureEnv(t *testing.T) {
	t.Helper()
	savedR, savedK := Release, UpdatePublicKey
	Release = "abc123"
	UpdatePublicKey = ""
	t.Cleanup(func() {
		Release, UpdatePublicKey = savedR, savedK
	})
}

// testConfig returns a fully-initialized config (the Config literal is
// exhaustruct-checked), with an empty Image so the release rule decides whether
// to auto-download.
func testConfig(server, image string) runner.Config {
	return runner.Config{
		Server: server, Repos: nil, Image: image,
		DataDir: "", GoModCache: "", NPMCache: "", BotEmail: "", Mode: "", MaxJobs: 0,
		OnLimit: "",
	}
}

func TestEnsureImageFetchesForARelease(t *testing.T) {
	ts, pubB64, _ := ensureServer(t)
	defer ts.Close()
	testEnsureEnv(t)
	UpdatePublicKey = pubB64

	require.NoError(t, ensureImage(
		context.Background(), testConfig(ts.URL, ""), &imageEnsureExec{inspects: 0}))
}

func TestEnsureImageSkipsWhenDisabled(t *testing.T) {
	testEnsureEnv(t)
	// Explicit override: never auto-download.
	require.NoError(t, ensureImage(context.Background(),
		testConfig("https://example.com/api", "local:v1"), &imageEnsureExec{inspects: 0}))
	// No signing key: rely on the local image.
	require.NoError(t, ensureImage(context.Background(),
		testConfig("https://example.com/api", ""), &imageEnsureExec{inspects: 0}))
}

func TestEnsureImageErrorsOnBadServerOrKey(t *testing.T) {
	testEnsureEnv(t)
	// Unparseable server.
	require.Error(t, ensureImage(context.Background(),
		testConfig("://bad", ""), &imageEnsureExec{inspects: 0}))
	// Invalid update key.
	UpdatePublicKey = "not-an-ed25519-key"
	require.Error(t, ensureImage(context.Background(),
		testConfig("https://example.com/api", ""), &imageEnsureExec{inspects: 0}))
}

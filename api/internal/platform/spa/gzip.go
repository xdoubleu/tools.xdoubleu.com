package spa

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// compressible are the text types worth gzipping; the rest (images, fonts,
// archives) are already compressed.
//
//nolint:gochecknoglobals //read-only allowlist
var compressible = map[string]bool{
	extJS: true, extMJS: true, ".css": true, ".svg": true, ".json": true,
	extManifest: true, ".txt": true,
}

type gzipEntry struct {
	modTime time.Time
	body    []byte
}

// gzipCache compresses each text file once and serves it to clients that
// accept gzip; the build is small, so the cache is bounded by the image.
type gzipCache struct {
	mu      sync.Mutex
	entries map[string]gzipEntry
}

func newGzipCache() gzipCache {
	return gzipCache{mu: sync.Mutex{}, entries: map[string]gzipEntry{}}
}

// serve writes the gzipped file and reports true, or reports false when the
// client or file type doesn't call for it.
func (c *gzipCache) serve(
	w http.ResponseWriter, r *http.Request, root *os.Root, name string,
	modTime time.Time,
) bool {
	if !compressible[path.Ext(name)] ||
		!strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}

	body, ok := c.load(root, name, modTime)
	if !ok {
		return false
	}

	hdr := w.Header()
	hdr.Set("Content-Encoding", "gzip")
	hdr.Add("Vary", "Accept-Encoding")
	http.ServeContent(w, r, name, modTime, bytes.NewReader(body))
	return true
}

func (c *gzipCache) load(
	root *os.Root, name string, modTime time.Time,
) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[name]; ok && e.modTime.Equal(modTime) {
		return e.body, true
	}

	raw, err := root.ReadFile(name)
	if err != nil {
		return nil, false
	}
	body, err := compress(raw)
	if err != nil {
		return nil, false
	}
	c.entries[name] = gzipEntry{modTime: modTime, body: body}
	return body, true
}

func compress(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(zw, bytes.NewReader(raw)); err != nil {
		return nil, err
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

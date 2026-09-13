//nolint:testpackage // testing unexported currentKEPUBConverterVersion
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// pipelineFiles lists the source files whose content determines the output of
// the PDF/EPUB-to-KEPUB conversion pipeline (goPDFConverter's PDF extraction
// and EPUB assembly stages). conversion.go itself is deliberately excluded:
// it hosts currentKEPUBConverterVersion and the versioning/caching control
// flow, not pipeline transform logic, so editing it alone (e.g. to bump the
// version) must not by itself trip this check.
//
// Add a file here whenever a new one is introduced that changes conversion
// output (e.g. a new conversion_pdfextract_*.go or conversion_epubbuild_*.go
// file) — otherwise a behavior change there would silently bypass this
// check the same way issue #612 described.
//
//nolint:gochecknoglobals // fixed test data, not mutable state
var pipelineFiles = []string{
	"conversion_epubbuild.go",
	"conversion_epubbuild_xhtml.go",
	"conversion_pdfextract.go",
	"conversion_pdfextract_bitmap.go",
	"conversion_pdfextract_chars.go",
	"conversion_pdfextract_columns.go",
	"conversion_pdfextract_images.go",
	"conversion_pdfextract_page.go",
	"conversion_pdfextract_paragraphs.go",
	"conversion_pdfextract_proofslug.go",
}

// pipelineFilesHashForVersion maps currentKEPUBConverterVersion to the sha256
// hash of pipelineFiles' concatenated content as of the commit that last set
// that version. Whenever a change to any pipelineFiles entry alters
// conversion output, currentKEPUBConverterVersion (conversion.go) must be
// bumped and the corresponding entry here updated to the new hash — in the
// same PR. This is what makes a missed bump (the actual gap behind issue
// #612: PR #620 fixed a PDF-pipeline bug without bumping the version, so
// already-converted books kept the old bug) fail CI instead of shipping
// silently.
//
//nolint:gochecknoglobals // fixed test data, not mutable state
var pipelineFilesHashForVersion = map[int16]string{
	1: "664a7e6eb4ef2d6bdae9142c00fd54a063fd311e64234208d1738925a689b545",
	2: "10627fd199c1279262f45866d3f2f8e2f5953782b7ceb24fd9ffd2c5958deb1f",
	3: "33a4312f7ea1b063766175392e2c5cde581abdc6bf90fc0193d94762401154e1",
	4: "5ba47eb03c8fbd7675b7524f5c23be5bdb593a645198f900127aae05e568b584",
	5: "5f1c6dc23f3b12bf016a2dfcf1a97e10616fd99cf0ffc21c39555cab96137880",
	6: "58887c35e02f6a5c8f53d59ae7b7a6b1afbdc900811cb419ab70162bed2ed894",
}

// hashPipelineFiles returns the sha256 hash of pipelineFiles' concatenated
// content, in the fixed order they're declared.
func hashPipelineFiles(t *testing.T) string {
	t.Helper()

	h := sha256.New()
	for _, name := range pipelineFiles {
		data, err := os.ReadFile(name)
		require.NoError(t, err, "read pipeline file %s", name)
		_, err = h.Write(data)
		require.NoError(t, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestConverterVersionBumpedWithPipelineChanges fails whenever pipelineFiles'
// content no longer matches the hash recorded for currentKEPUBConverterVersion,
// signalling that the pipeline changed without a version bump (or the bump
// happened without recording the new hash). Either way, EnsureKEPUB won't
// regenerate already-converted books for a change this test doesn't yet know
// about, so it must fail loudly until both are updated together.
func TestConverterVersionBumpedWithPipelineChanges(t *testing.T) {
	got := hashPipelineFiles(t)

	want, ok := pipelineFilesHashForVersion[currentKEPUBConverterVersion]
	require.True(t, ok,
		"no recorded pipeline hash for currentKEPUBConverterVersion=%d; "+
			"add pipelineFilesHashForVersion[%d] = %q to converter_version_check_test.go",
		currentKEPUBConverterVersion, currentKEPUBConverterVersion, got,
	)

	require.Equal(t, want, got,
		"conversion pipeline files changed (see pipelineFiles in "+
			"converter_version_check_test.go) but currentKEPUBConverterVersion "+
			"(conversion.go) was not bumped to match. Bump the version AND set "+
			"pipelineFilesHashForVersion[<new version>] = %q, so EnsureKEPUB "+
			"regenerates already-converted books.", got,
	)
}

// TestHashPipelineFiles_ChangesWithContent proves hashPipelineFiles is
// content-sensitive: two different byte sequences must not hash the same,
// otherwise TestConverterVersionBumpedWithPipelineChanges above could never
// detect a real pipeline change.
func TestHashPipelineFiles_ChangesWithContent(t *testing.T) {
	t.Chdir(t.TempDir())

	require.NoError(t, os.WriteFile("a.go", []byte("package services\n// v1\n"), 0o600))
	before := hashSingleFile(t, "a.go")

	require.NoError(t, os.WriteFile("a.go", []byte("package services\n// v2\n"), 0o600))
	after := hashSingleFile(t, "a.go")

	require.NotEqual(t, before, after,
		"hash must change when pipeline file content changes")
}

// hashSingleFile hashes one file the same way hashPipelineFiles does, for use
// against a temp directory in TestHashPipelineFiles_ChangesWithContent.
func hashSingleFile(t *testing.T, name string) string {
	t.Helper()

	h := sha256.New()
	data, err := os.ReadFile(name)
	require.NoError(t, err)
	_, err = h.Write(data)
	require.NoError(t, err)
	return hex.EncodeToString(h.Sum(nil))
}

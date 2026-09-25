//nolint:testpackage // testing unexported currentKEPUBConverterVersion
package services

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// pipelineFiles are the sources that determine conversion output. Add any new
// conversion_pdfextract_*/conversion_epubbuild_* file. conversion.go is
// excluded so bumping the version alone doesn't trip the check.
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

// pipelineFilesHashForVersion maps each converter version to the hash of
// pipelineFiles when it was set. A pipeline change must bump
// currentKEPUBConverterVersion and record the new hash in the same PR, so a
// missed bump fails CI.
//
//nolint:gochecknoglobals // fixed test data, not mutable state
var pipelineFilesHashForVersion = map[int16]string{
	1:  "664a7e6eb4ef2d6bdae9142c00fd54a063fd311e64234208d1738925a689b545",
	2:  "10627fd199c1279262f45866d3f2f8e2f5953782b7ceb24fd9ffd2c5958deb1f",
	3:  "33a4312f7ea1b063766175392e2c5cde581abdc6bf90fc0193d94762401154e1",
	4:  "5ba47eb03c8fbd7675b7524f5c23be5bdb593a645198f900127aae05e568b584",
	5:  "5f1c6dc23f3b12bf016a2dfcf1a97e10616fd99cf0ffc21c39555cab96137880",
	6:  "58887c35e02f6a5c8f53d59ae7b7a6b1afbdc900811cb419ab70162bed2ed894",
	7:  "f3f20503d27a99401a8559c056e98ef44b7294f7cad19568544ea012dd63a5a5",
	8:  "0ac51cc72fd880171942d10d32c6f70f7527d0d6d8c8aced1fb804ffd7ebe40f",
	9:  "997946977fc79fdbc21d1b77851bd88189b542c4e88d6fd40d6abacdb58005b7",
	10: "764f8c35bf4eb890606ddc15ad1f11d19e4fdafc016e37f9b09ada35afe722a8",
	11: "7edf20385a07433654781d7987b6e1feaed07c24047c90b2ac2c746a3b160126",
	12: "65f5b8506c78773d4f83fe8b12a2f0962c1fb806cfd722498a1037675a12019a",
}

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

// TestConverterVersionBumpedWithPipelineChanges fails when pipelineFiles no
// longer match the hash for the current version.
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

// TestHashPipelineFiles_ChangesWithContent checks hashing is content-sensitive.
func TestHashPipelineFiles_ChangesWithContent(t *testing.T) {
	t.Chdir(t.TempDir())

	require.NoError(t, os.WriteFile("a.go", []byte("package services\n// v1\n"), 0o600))
	before := hashSingleFile(t, "a.go")

	require.NoError(t, os.WriteFile("a.go", []byte("package services\n// v2\n"), 0o600))
	after := hashSingleFile(t, "a.go")

	require.NotEqual(t, before, after,
		"hash must change when pipeline file content changes")
}

func hashSingleFile(t *testing.T, name string) string {
	t.Helper()

	h := sha256.New()
	data, err := os.ReadFile(name)
	require.NoError(t, err)
	_, err = h.Write(data)
	require.NoError(t, err)
	return hex.EncodeToString(h.Sum(nil))
}

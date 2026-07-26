//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pdfLocalDir holds real PDFs for the `make pdf/check` manual verification
// step (issue #588's "real-world check"). It's gitignored — no PDF is ever
// committed — and populated locally by whoever runs the check.
const pdfLocalDir = "testdata/pdf.local"

// TestPDFCheck converts every PDF in pdfLocalDir to EPUB in a scratch
// subdirectory, for manual inspection. It's a normal test (so `make test`
// exercises it harmlessly) but skips immediately when pdfLocalDir doesn't
// exist or has no PDFs, which is always true in CI since the directory is
// gitignored.
func TestPDFCheck(t *testing.T) {
	entries, err := os.ReadDir(pdfLocalDir)
	if err != nil {
		t.Skipf("no local PDFs to check (%s): %v", pdfLocalDir, err)
	}

	outDir := filepath.Join(pdfLocalDir, "output")
	if err = os.MkdirAll(outDir, 0o750); err != nil {
		t.Fatalf("create output dir: %v", err)
	}

	converted := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			continue
		}
		converted++

		inPath := filepath.Join(pdfLocalDir, entry.Name())
		outPath := filepath.Join(
			outDir,
			strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))+".epub",
		)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		start := time.Now()
		convErr := goPDFConverter(ctx, inPath, outPath)
		elapsed := time.Since(start)
		cancel()

		if convErr != nil {
			t.Errorf(
				"%s: conversion failed after %s: %v",
				entry.Name(),
				elapsed,
				convErr,
			)
			continue
		}
		info, statErr := os.Stat(outPath)
		if statErr != nil {
			t.Errorf("%s: converted but output missing: %v", entry.Name(), statErr)
			continue
		}
		t.Logf("%s -> %s (%d bytes) in %s", entry.Name(), outPath, info.Size(), elapsed)
	}

	if converted == 0 {
		t.Skipf("no .pdf files found in %s", pdfLocalDir)
	}
}

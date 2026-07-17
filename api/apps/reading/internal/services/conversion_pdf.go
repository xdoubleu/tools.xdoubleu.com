package services

import (
	"context"
	"fmt"
	"os/exec"
)

// calibreSem limits ebook-convert to one concurrent subprocess across every
// conversion kind (PDF→EPUB and article HTML→EPUB share the slot).
// Calibre is memory-hungry; one at a time prevents OOM on small instances.
// The channel is empty-at-start with capacity 1: acquire by sending a token,
// release by receiving it — no init() required.
//
//nolint:gochecknoglobals // package-level semaphore; no per-instance state needed
var calibreSem = make(chan struct{}, 1)

// runEbookConvert acquires the Calibre slot and shells out to ebook-convert
// with the given input/output paths and extra arguments.
func runEbookConvert(ctx context.Context, inPath, outPath string,
	extraArgs ...string,
) error {
	// Acquire the semaphore (blocks if another conversion is running).
	select {
	case calibreSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-calibreSem }()

	args := append([]string{inPath, outPath}, extraArgs...)
	cmd := exec.CommandContext(ctx, "ebook-convert", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ebook-convert: %w\noutput: %s", err, out)
	}
	return nil
}

// calibrePDFConverter shells out to ebook-convert (Calibre) to produce an
// EPUB from a PDF.
func calibrePDFConverter(
	ctx context.Context,
	inPath string,
	outPath string,
) error {
	return runEbookConvert(ctx, inPath, outPath)
}

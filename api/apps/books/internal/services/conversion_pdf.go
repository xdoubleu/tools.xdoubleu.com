package services

import (
	"context"
	"fmt"
	"os/exec"
)

// pdfConvertSem limits ebook-convert to one concurrent subprocess.
// Calibre is memory-hungry; one at a time prevents OOM on small instances.
// The channel is empty-at-start with capacity 1: acquire by sending a token,
// release by receiving it — no init() required.
//
//nolint:gochecknoglobals // package-level semaphore; no per-instance state needed
var pdfConvertSem = make(chan struct{}, 1)

// calibrePDFConverter shells out to ebook-convert (Calibre) to produce an EPUB
// from a PDF. It blocks until the semaphore is acquired, ensuring only one
// ebook-convert process runs at a time.
func calibrePDFConverter(
	ctx context.Context,
	inPath string,
	outPath string,
) error {
	// Acquire the semaphore (blocks if another conversion is running).
	select {
	case pdfConvertSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-pdfConvertSem }()

	cmd := exec.CommandContext(ctx, "ebook-convert", inPath, outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ebook-convert: %w\noutput: %s", err, out)
	}
	return nil
}

package conversion

import (
	"sync"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// analysisPDFiumMemoryLimitPages caps the figure-analysis pool's wazero linear
// memory (64 KiB/page) at 512 MiB: opening a pathological PDF with a huge
// embedded image otherwise grows it toward the 4 GiB wazero default (the
// #2520 api restart loop), so over-limit PDFs fail cleanly and are skipped.
// Kept separate from the version-guarded conversion pool in
// conversion_pdfextract.go to avoid a needless re-conversion.
const analysisPDFiumMemoryLimitPages = 8192

// analysisPDFiumPool is a lazily-initialized, package-level pdfium wasm pool
// for the figure-analysis path only, with a capped linear memory so one
// pathological PDF can't take the process down.
//
//nolint:gochecknoglobals // lazily-initialized singleton wasm runtime pool
var (
	analysisPoolOnce sync.Once
	analysisPool     pdfium.Pool
	//nolint:errname // cached init error, not a sentinel
	analysisPoolErr error
)

// getAnalysisPDFiumPool returns the figure-analysis pool with the memory cap.
// Keep the exception-handling core feature go-pdfium requires for a custom
// RuntimeConfig.
func getAnalysisPDFiumPool() (pdfium.Pool, error) {
	analysisPoolOnce.Do(func() {
		analysisPool, analysisPoolErr = webassembly.Init(
			webassembly.Config{ //nolint:exhaustruct // zero-value defaults are correct
				MinIdle:  0,
				MaxIdle:  1,
				MaxTotal: 1,
				RuntimeConfig: wazero.NewRuntimeConfig().
					WithMemoryLimitPages(analysisPDFiumMemoryLimitPages).
					WithCoreFeatures(
						api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling,
					),
			},
		)
	})
	return analysisPool, analysisPoolErr
}

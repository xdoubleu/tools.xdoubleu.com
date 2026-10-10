// Loads the readers' code and the pdf.js worker while online, so the service
// worker caches them and a downloaded book opens offline before its reader was
// ever used. Browser-only.
import { pdfWorkerUrl } from './pdf'

let warmed = false

/**
 * Repeats until every load succeeds once in this page load; a no-op without a
 * controlling service worker.
 */
export async function warmReaderModules(): Promise<void> {
  if (warmed || !navigator.serviceWorker?.controller) return
  try {
    await Promise.all([
      import('@/components/books/reader/ReadiumBookReader'),
      import('@/components/books/reader/PdfBookReader'),
      fetch(pdfWorkerUrl()).then((res) => {
        if (!res.ok) throw new Error(`pdf.js worker request failed: ${res.status}`)
      })
    ])
    warmed = true
  } catch {
    // Retried on the next sync.
  }
}

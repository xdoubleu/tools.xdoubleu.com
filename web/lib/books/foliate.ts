// foliate-js is served unbundled from public/foliate-js/ (scripts/copy-foliate.mjs)
// so its pdf.js can resolve the worker, cmaps and fonts against its own
// import.meta.url. Browser-only: load it from an effect.
import type { ReaderSection } from './readerPosition'

const FOLIATE_VIEW_URL = '/foliate-js/view.js'

export interface FoliateTocItem {
  label: string
  /** Missing on grouping entries that don't link anywhere. */
  href?: string
  subitems?: FoliateTocItem[]
}

/** `relocate` event detail; `section.current` is the spine index (the page for a PDF). */
export interface FoliateRelocateDetail {
  fraction?: number
  section?: { current: number; total: number }
  cfi?: string
  tocItem?: { label?: string; href?: string }
  /** Visible range in the section document. */
  range?: Range
}

interface FoliateViewEventMap extends HTMLElementEventMap {
  relocate: CustomEvent<FoliateRelocateDetail>
  /** A section's document finished loading into its iframe. */
  load: CustomEvent<{ doc: Document; index: number }>
}

/** The subset of foliate-js's `<foliate-view>` the reader uses. */
export interface FoliateView extends HTMLElement {
  addEventListener<K extends keyof FoliateViewEventMap>(
    type: K,
    listener: (this: FoliateView, ev: FoliateViewEventMap[K]) => unknown,
    options?: boolean | AddEventListenerOptions
  ): void
  addEventListener(
    type: string,
    listener: EventListenerOrEventListenerObject,
    options?: boolean | AddEventListenerOptions
  ): void
  book?: { toc?: FoliateTocItem[]; sections?: ReaderSection[]; destroy?: () => void }
  isFixedLayout: boolean
  renderer?: {
    addEventListener?: (
      type: 'relocate',
      listener: (e: CustomEvent<{ reason?: string }>) => void,
      options?: AddEventListenerOptions
    ) => void
    setStyles?: (css: string) => void
    goTo: (target: { index: number; anchor?: (doc: Document) => Range }) => Promise<unknown>
  }
  open(book: string | Blob): Promise<void>
  init(options: { lastLocation?: string; showTextStart?: boolean }): Promise<void>
  goTo(target: string | number): Promise<unknown>
  goToFraction(fraction: number): Promise<void>
  goLeft(): unknown
  goRight(): unknown
  close(): void
}

const importModule = (url: string) =>
  import(/* webpackIgnore: true */ /* turbopackIgnore: true */ url)

// The modules an EPUB or PDF needs, cmaps and standard fonts aside (fetched
// per PDF on demand). A path missing after a pin bump just isn't warmed.
const READER_MODULES = [
  'view.js',
  'epubcfi.js',
  'progress.js',
  'overlayer.js',
  'text-walker.js',
  'vendor/zip.js',
  'epub.js',
  'paginator.js',
  'fixed-layout.js',
  'pdf.js',
  'vendor/pdfjs/pdf.mjs',
  'vendor/pdfjs/pdf.worker.mjs',
  'vendor/pdfjs/text_layer_builder.css',
  'vendor/pdfjs/annotation_layer_builder.css'
].map((path) => `/foliate-js/${path}`)

let warmed = false

/**
 * Fetches the reader's modules through the service worker, which caches
 * them, so a downloaded book opens offline before the reader was ever used
 * online. Repeats until every fetch succeeds once in this page load; a no-op
 * without a controlling worker.
 */
export async function warmReaderModules(): Promise<void> {
  if (warmed || !navigator.serviceWorker?.controller) return
  let ok = true
  for (const url of READER_MODULES) {
    const res = await fetch(url).catch(() => null)
    if (!res?.ok) ok = false
  }
  warmed = ok
}

export async function createFoliateView(load: (url: string) => Promise<unknown> = importModule) {
  await load(FOLIATE_VIEW_URL)
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- view.js registers <foliate-view> as its View class
  return document.createElement('foliate-view') as FoliateView
}

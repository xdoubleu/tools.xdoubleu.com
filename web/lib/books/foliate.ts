// foliate-js is served unbundled from public/foliate-js/ (scripts/copy-foliate.mjs)
// so its pdf.js can resolve the worker, cmaps and fonts against its own
// import.meta.url. Browser-only: load it from an effect.
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
  book?: { toc?: FoliateTocItem[]; destroy?: () => void }
  isFixedLayout: boolean
  renderer?: { setStyles?: (css: string) => void }
  open(book: string | Blob): Promise<void>
  init(options: { lastLocation?: string; showTextStart?: boolean }): Promise<void>
  goTo(target: string | number): Promise<unknown>
  goLeft(): unknown
  goRight(): unknown
  close(): void
}

const importModule = (url: string) =>
  import(/* webpackIgnore: true */ /* turbopackIgnore: true */ url)

export async function createFoliateView(load: (url: string) => Promise<unknown> = importModule) {
  await load(FOLIATE_VIEW_URL)
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- view.js registers <foliate-view> as its View class
  return document.createElement('foliate-view') as FoliateView
}

// pdf.js glue for the PDF reader. Browser-only: load it from an effect.
import type { PDFDocumentProxy } from 'pdfjs-dist'
import type { ReaderTocItem } from './readerPosition'

const PAGE_HREF = '#page='

/** The contents entry for a page; `pageOf` reads it back. */
const pageHref = (page: number) => `${PAGE_HREF}${page}`

export function pageOf(href: string): number | null {
  if (!href.startsWith(PAGE_HREF)) return null
  const page = Number(href.slice(PAGE_HREF.length))
  return Number.isInteger(page) && page > 0 ? page : null
}

/** Where the pdf.js worker is served from. */
export const pdfWorkerUrl = () =>
  new URL('pdfjs-dist/legacy/build/pdf.worker.min.mjs', import.meta.url).toString()

/** Opens a PDF from its bytes, or from a URL fetched with the session cookie. */
export async function openPdf(
  file: Blob | string
): Promise<{ pdf: PDFDocumentProxy; destroy: () => Promise<void> }> {
  const pdfjs = await import('pdfjs-dist/legacy/build/pdf.mjs')
  pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerUrl()
  const source =
    typeof file === 'string'
      ? { url: file, withCredentials: true }
      : { data: new Uint8Array(await file.arrayBuffer()) }
  const task = pdfjs.getDocument(source)
  return { pdf: await task.promise, destroy: () => task.destroy() }
}

interface OutlineNode {
  title: string
  dest: string | unknown[] | null
  items: OutlineNode[]
}

async function pageOfDest(
  pdf: PDFDocumentProxy,
  dest: OutlineNode['dest']
): Promise<number | null> {
  try {
    const explicit = typeof dest === 'string' ? await pdf.getDestination(dest) : dest
    const target: unknown = explicit?.[0]
    if (typeof target === 'number') return target + 1
    if (target && typeof target === 'object') {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a RefProxy from the destination array
      return (await pdf.getPageIndex(target as Parameters<typeof pdf.getPageIndex>[0])) + 1
    }
  } catch {
    // An unresolvable destination is just not linkable.
  }
  return null
}

async function outlineItems(pdf: PDFDocumentProxy, nodes: OutlineNode[]): Promise<ReaderTocItem[]> {
  return Promise.all(
    nodes.map(async (n) => {
      const page = await pageOfDest(pdf, n.dest)
      return {
        label: n.title,
        href: page ? pageHref(page) : undefined,
        subitems: n.items.length ? await outlineItems(pdf, n.items) : undefined
      }
    })
  )
}

/** The PDF's outline as contents entries linking to `#page=N`. */
export async function pdfOutline(pdf: PDFDocumentProxy): Promise<ReaderTocItem[]> {
  const outline = await pdf.getOutline().catch(() => null)
  return outline ? outlineItems(pdf, outline) : []
}

/** The title of the last entry at or before `page`, in document order. */
export function tocLabelForPage(items: ReaderTocItem[], page: number): string | undefined {
  let label: string | undefined
  for (const item of items) {
    const at = item.href ? pageOf(item.href) : null
    if (at !== null && at <= page) label = item.label
    label = tocLabelForPage(item.subitems ?? [], page) ?? label
  }
  return label
}

/** Draws `pageNumber` scaled to fit within `width` x `height`; resolves false if cancelled. */
export async function renderPage(
  pdf: PDFDocumentProxy,
  pageNumber: number,
  canvas: HTMLCanvasElement,
  width: number,
  height: number,
  signal?: AbortSignal
): Promise<boolean> {
  const page = await pdf.getPage(pageNumber)
  const natural = page.getViewport({ scale: 1 })
  const scale = Math.min(width / natural.width, height / natural.height)
  const ratio = window.devicePixelRatio || 1
  const viewport = page.getViewport({ scale: scale * ratio })
  canvas.width = Math.floor(viewport.width)
  canvas.height = Math.floor(viewport.height)
  canvas.style.width = `${Math.floor(viewport.width / ratio)}px`
  canvas.style.height = `${Math.floor(viewport.height / ratio)}px`
  if (signal?.aborted) return false
  const task = page.render({ canvas, viewport })
  signal?.addEventListener('abort', () => task.cancel())
  try {
    await task.promise
    return true
  } catch (err) {
    if (err instanceof Error && err.name === 'RenderingCancelledException') return false
    throw err
  }
}

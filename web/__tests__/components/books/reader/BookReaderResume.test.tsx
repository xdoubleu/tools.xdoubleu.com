import React from 'react'
import { act, render, waitFor } from '@testing-library/react'
import type { ReaderResume, ReaderSection } from '@/lib/books/readerPosition'

const mockCreateFoliateView = jest.fn()

jest.mock('@/lib/books/foliate', () => ({
  createFoliateView: () => mockCreateFoliateView()
}))

import BookReader from '@/components/books/reader/BookReader'

const EPUB_SECTIONS: ReaderSection[] = [{ id: 'OEBPS/cover.xhtml' }, { id: 'OEBPS/ch1.xhtml' }]
const PDF_SECTIONS: ReaderSection[] = [{ id: 0 }, { id: 1 }, { id: 2 }]

// foliate re-emits a relocate from its renderer, so its detail carries the
// renderer's fraction/index.
type FoliateRelocate = CustomEvent<{ fraction?: number; index?: number; range?: Range }>

function makeView(sections: ReaderSection[] = EPUB_SECTIONS) {
  const view = Object.assign(document.createElement('div'), {
    isFixedLayout: typeof sections[0]?.id === 'number',
    book: undefined as { toc: never[]; sections: ReaderSection[] } | undefined,
    renderer: Object.assign(new EventTarget(), {
      setStyles: jest.fn(),
      goTo: jest.fn<Promise<void>, [{ index: number; anchor?: (doc: Document) => Range }]>(
        async () => {}
      )
    }),
    open: jest.fn(async () => {
      view.book = { toc: [], sections }
      // Like foliate's view: re-emit the renderer's relocate, carrying its
      // fraction and index rather than a reason.
      view.renderer?.addEventListener?.('relocate', (e: Event) => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- foliate's relocate detail
        const { fraction, index, range } = (e as unknown as FoliateRelocate).detail
        view.dispatchEvent(
          new CustomEvent('relocate', {
            detail: { fraction, section: { current: index, total: sections.length }, range }
          })
        )
      })
    }),
    init: jest.fn(async () => {}),
    goTo: jest.fn(async () => {}),
    goToFraction: jest.fn(async () => {}),
    goLeft: jest.fn(),
    goRight: jest.fn(),
    close: jest.fn()
  })
  return view
}

type FakeView = ReturnType<typeof makeView>

// A genuine page turn: the renderer relocates with a read reason, which the
// capture listener and the view's re-emit turn into a reported location.
function pageTurn(
  view: FakeView,
  { index = 0, fraction = 0.5, range }: { index?: number; fraction?: number; range?: Range }
) {
  act(() => {
    ;(view.renderer as EventTarget).dispatchEvent(
      new CustomEvent('relocate', {
        detail: { reason: 'page', index, fraction, range }
      })
    )
  })
}

async function renderReader(
  view: FakeView,
  initialPosition?: ReaderResume,
  onRelocate = jest.fn()
) {
  mockCreateFoliateView.mockResolvedValue(view)
  render(
    <BookReader
      file="https://r2/book"
      title="Dune"
      onClose={jest.fn()}
      onRelocate={onRelocate}
      initialPosition={initialPosition}
    />
  )
  await waitFor(() => expect(view.open).toHaveBeenCalled())
  await act(async () => {})
  return onRelocate
}

beforeEach(() => {
  mockCreateFoliateView.mockReset()
  localStorage.clear()
})

describe('BookReader resume', () => {
  it('opens the stored EPUB section at the stored text offset', async () => {
    const view = makeView()
    await renderReader(view, { position: { href: 'OEBPS/ch1.xhtml', offset: 6 }, percent: 40 })
    expect(view.renderer.goTo).toHaveBeenCalledWith({ index: 1, anchor: expect.any(Function) })
    const anchor = view.renderer.goTo.mock.calls[0]![0].anchor!
    const doc = document.implementation.createHTMLDocument('ch1')
    doc.body.innerHTML = '<p>Call me Ishmael.</p>'
    expect(anchor(doc).startOffset).toBe(6)
    expect(view.init).not.toHaveBeenCalled()
  })

  it('opens a PDF at the stored page', async () => {
    const view = makeView(PDF_SECTIONS)
    await renderReader(view, { position: { page: 3 }, percent: 90 })
    expect(view.renderer.goTo).toHaveBeenCalledWith({ index: 2 })
  })

  it('seeks to the stored percent without a usable position', async () => {
    const view = makeView()
    await renderReader(view, { percent: 35 })
    expect(view.goToFraction).toHaveBeenCalledWith(0.35)
    expect(view.init).not.toHaveBeenCalled()
  })

  it('opens at the start with nothing stored', async () => {
    const view = makeView()
    await renderReader(view, { percent: 0 })
    expect(view.init).toHaveBeenCalledWith({})
  })

  it('opens at the start when the seek fails', async () => {
    const view = makeView()
    view.renderer.goTo.mockRejectedValue(new Error('bad anchor'))
    await renderReader(view, { position: { href: 'OEBPS/ch1.xhtml', offset: 6 }, percent: 40 })
    expect(view.init).toHaveBeenCalledWith({})
  })

  it('reports relocates only after the book has opened at its position', async () => {
    const view = makeView()
    const onRelocate = jest.fn()
    view.renderer.goTo.mockImplementation(async () => {
      view.dispatchEvent(new CustomEvent('relocate', { detail: { fraction: 0.4 } }))
    })
    await renderReader(
      view,
      { position: { href: 'OEBPS/ch1.xhtml', offset: 6 }, percent: 40 },
      onRelocate
    )
    expect(onRelocate).not.toHaveBeenCalled()

    const doc = document.implementation.createHTMLDocument('ch1')
    doc.body.innerHTML = '<p>Call me Ishmael.</p>'
    const range = doc.createRange()
    range.setStart(doc.querySelector('p')!.firstChild!, 8)
    pageTurn(view, { index: 1, fraction: 0.45, range })
    expect(onRelocate).toHaveBeenCalledWith(
      expect.objectContaining({
        fraction: 0.45,
        position: { href: 'OEBPS/ch1.xhtml', offset: 8 }
      })
    )
  })

  it('does not report an open settle, so opening never saves a 0% position', async () => {
    const view = makeView()
    const onRelocate = await renderReader(view)
    // The reader can still be settling at the start after it has opened; a
    // raw relocate and a seek ('navigation') relocate both precede the first
    // real page turn and must not reach onRelocate (which saves the position).
    ;(view.renderer as EventTarget).dispatchEvent(
      new CustomEvent('relocate', {
        detail: { reason: 'navigation', index: 0, fraction: 0 }
      })
    )
    act(() => {
      view.dispatchEvent(new CustomEvent('relocate', { detail: { fraction: 0 } }))
    })
    expect(onRelocate).not.toHaveBeenCalled()
  })

  it('does not report a reflow around the same spot', async () => {
    const view = makeView()
    const renderer = Object.assign(new EventTarget(), view.renderer)
    view.renderer = renderer
    // Like foliate's view: re-emit the renderer's relocate, dropping its reason.
    view.open.mockImplementation(async () => {
      view.book = { toc: [], sections: EPUB_SECTIONS }
      renderer.addEventListener('relocate', () => {
        view.dispatchEvent(new CustomEvent('relocate', { detail: { fraction: 0.4 } }))
      })
    })
    const onRelocate = await renderReader(view)
    const fromRenderer = (reason: string) =>
      act(() => {
        renderer.dispatchEvent(new CustomEvent('relocate', { detail: { reason } }))
      })

    fromRenderer('anchor')
    expect(onRelocate).not.toHaveBeenCalled()
    fromRenderer('page')
    expect(onRelocate).toHaveBeenCalledTimes(1)
  })

  it('reports PDF pages as positions', async () => {
    const view = makeView(PDF_SECTIONS)
    const onRelocate = await renderReader(view)
    pageTurn(view, { index: 1, fraction: 0.5 })
    expect(onRelocate).toHaveBeenCalledWith(expect.objectContaining({ position: { page: 2 } }))
  })

  it('reports no position for a book without sections', async () => {
    const view = makeView([])
    const onRelocate = await renderReader(view)
    pageTurn(view, { fraction: 0.2 })
    expect(onRelocate).toHaveBeenCalledWith(expect.objectContaining({ position: undefined }))
  })

  it('opens at the start when a position has no renderer to seek with', async () => {
    const view = makeView()
    Object.assign(view, { renderer: undefined })
    await renderReader(view, { position: { href: 'OEBPS/ch1.xhtml', offset: 6 }, percent: 40 })
    expect(view.init).toHaveBeenCalledWith({})
  })
})

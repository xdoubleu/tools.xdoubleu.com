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

function makeView(sections: ReaderSection[] = EPUB_SECTIONS) {
  const view = Object.assign(document.createElement('div'), {
    isFixedLayout: typeof sections[0]?.id === 'number',
    book: undefined as { toc: never[]; sections: ReaderSection[] } | undefined,
    renderer: {
      setStyles: jest.fn(),
      goTo: jest.fn<Promise<void>, [{ index: number; anchor?: (doc: Document) => Range }]>(
        async () => {}
      )
    },
    open: jest.fn(async () => {
      view.book = { toc: [], sections }
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

function relocate(view: FakeView, detail: Record<string, unknown>) {
  act(() => {
    view.dispatchEvent(new CustomEvent('relocate', { detail }))
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
      url="https://r2/book"
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
    relocate(view, { fraction: 0.45, section: { current: 1, total: 2 }, range })
    expect(onRelocate).toHaveBeenCalledWith(
      expect.objectContaining({
        fraction: 0.45,
        position: { href: 'OEBPS/ch1.xhtml', offset: 8 }
      })
    )
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
    relocate(view, { fraction: 0.5, section: { current: 1, total: 3 } })
    expect(onRelocate).toHaveBeenCalledWith(expect.objectContaining({ position: { page: 2 } }))
  })

  it('reports no position for a book without sections', async () => {
    const view = makeView()
    view.open.mockImplementation(async () => {})
    const onRelocate = await renderReader(view)
    relocate(view, { fraction: 0.2 })
    expect(onRelocate).toHaveBeenCalledWith(expect.objectContaining({ position: undefined }))
  })

  it('opens at the start when a position has no renderer to seek with', async () => {
    const view = makeView()
    Object.assign(view, { renderer: undefined })
    await renderReader(view, { position: { href: 'OEBPS/ch1.xhtml', offset: 6 }, percent: 40 })
    expect(view.init).toHaveBeenCalledWith({})
  })
})

import React from 'react'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import type { FoliateTocItem } from '@/lib/books/foliate'

const mockCreateFoliateView = jest.fn()

jest.mock('@/lib/books/foliate', () => ({
  createFoliateView: () => mockCreateFoliateView()
}))

import BookReader from '@/components/books/reader/BookReader'

function makeView({
  toc = [],
  fixedLayout = false,
  openError
}: { toc?: FoliateTocItem[]; fixedLayout?: boolean; openError?: Error } = {}) {
  const view = Object.assign(document.createElement('div'), {
    isFixedLayout: fixedLayout,
    book: undefined as { toc: FoliateTocItem[] } | undefined,
    renderer: { setStyles: jest.fn() },
    open: jest.fn(async () => {
      if (openError) throw openError
      view.book = { toc }
    }),
    init: jest.fn(async () => {}),
    goTo: jest.fn(async () => {}),
    goLeft: jest.fn(),
    goRight: jest.fn(),
    close: jest.fn()
  })
  view.getBoundingClientRect = () => ({
    x: 0,
    y: 0,
    left: 0,
    top: 0,
    width: 300,
    height: 600,
    right: 300,
    bottom: 600,
    toJSON: () => ({})
  })
  return view
}

type FakeView = ReturnType<typeof makeView>

const URL_ = 'https://r2.example.com/book.epub?sig=1'

async function renderReader(
  view: FakeView,
  props: Partial<React.ComponentProps<typeof BookReader>> = {}
) {
  mockCreateFoliateView.mockResolvedValue(view)
  const onClose = jest.fn()
  const utils = render(<BookReader url={URL_} title="Dune" onClose={onClose} {...props} />)
  await waitFor(() => expect(view.init).toHaveBeenCalled())
  return { ...utils, onClose }
}

function relocate(view: FakeView, detail: Record<string, unknown>) {
  act(() => {
    view.dispatchEvent(new CustomEvent('relocate', { detail }))
  })
}

// foliate-js renders each section in an iframe.
function loadSection(view: FakeView) {
  const frame = document.createElement('iframe')
  document.body.append(frame)
  const doc = frame.contentDocument!
  act(() => {
    view.dispatchEvent(new CustomEvent('load', { detail: { doc, index: 0 } }))
  })
  return doc
}

describe('BookReader', () => {
  beforeEach(() => {
    mockCreateFoliateView.mockReset()
    localStorage.clear()
  })

  it('opens the file from the start', async () => {
    const view = makeView()
    await renderReader(view)
    expect(view.open).toHaveBeenCalledWith(URL_)
    expect(view.init).toHaveBeenCalledWith({})
    expect(screen.queryByText(/Loading book/)).not.toBeInTheDocument()
    expect(screen.getByText('Dune')).toBeInTheDocument()
  })

  it('shows an error when the file fails to open', async () => {
    const view = makeView({ openError: new Error('403 Forbidden') })
    mockCreateFoliateView.mockResolvedValue(view)
    render(<BookReader url={URL_} title="Dune" onClose={jest.fn()} />)
    expect(await screen.findByText('Failed to load book.')).toBeInTheDocument()
    expect(view.init).not.toHaveBeenCalled()
  })

  it('shows an error when foliate-js fails to load', async () => {
    mockCreateFoliateView.mockRejectedValue(new Error('chunk failed'))
    render(<BookReader url={URL_} title="Dune" onClose={jest.fn()} />)
    expect(await screen.findByText('Failed to load book.')).toBeInTheDocument()
  })

  it('closes from the close control', async () => {
    const { onClose } = await renderReader(makeView())
    fireEvent.click(screen.getByRole('button', { name: 'Close reader' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('tears the view down on unmount', async () => {
    const view = makeView()
    const { unmount } = await renderReader(view)
    unmount()
    expect(view.close).toHaveBeenCalled()
  })

  it('shows progress from the relocate fraction and reports the location', async () => {
    const view = makeView()
    const onRelocate = jest.fn()
    await renderReader(view, { onRelocate })
    relocate(view, {
      fraction: 0.4237,
      section: { current: 3, total: 10 },
      cfi: 'epubcfi(/6/8!/4/2)',
      tocItem: { label: 'Chapter 2', href: 'ch2.xhtml' }
    })
    const bar = screen.getByRole('progressbar')
    expect(bar).toHaveAttribute('aria-valuenow', '42')
    expect(screen.getByText('42%')).toBeInTheDocument()
    expect(screen.getByText('Chapter 2')).toBeInTheDocument()
    expect(onRelocate).toHaveBeenCalledWith({
      fraction: 0.4237,
      section: 3,
      cfi: 'epubcfi(/6/8!/4/2)',
      tocLabel: 'Chapter 2',
      tocHref: 'ch2.xhtml'
    })
  })

  it('turns pages with the arrow keys', async () => {
    const view = makeView()
    await renderReader(view)
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    fireEvent.keyDown(window, { key: 'ArrowLeft' })
    expect(view.goRight).toHaveBeenCalledTimes(1)
    expect(view.goLeft).toHaveBeenCalledTimes(1)
  })

  it('turns pages on taps near the edges of a loaded section', async () => {
    const view = makeView()
    await renderReader(view)
    const doc = loadSection(view)
    fireEvent.click(doc.body, { clientX: 280 })
    fireEvent.click(doc.body, { clientX: 20 })
    fireEvent.click(doc.body, { clientX: 150 })
    expect(view.goRight).toHaveBeenCalledTimes(1)
    expect(view.goLeft).toHaveBeenCalledTimes(1)
  })

  it('leaves link taps to foliate-js', async () => {
    const view = makeView()
    await renderReader(view)
    const doc = loadSection(view)
    const link = doc.createElement('a')
    link.setAttribute('href', '#note')
    doc.body.append(link)
    fireEvent.click(link, { clientX: 290 })
    expect(view.goRight).not.toHaveBeenCalled()
  })

  it('turns PDF pages on swipe', async () => {
    const view = makeView({ fixedLayout: true })
    await renderReader(view)
    const doc = loadSection(view)
    fireEvent.touchStart(doc.body, { changedTouches: [{ screenX: 200, screenY: 100 }] })
    fireEvent.touchEnd(doc.body, { changedTouches: [{ screenX: 100, screenY: 105 }] })
    expect(view.goRight).toHaveBeenCalledTimes(1)
  })

  it('leaves swipes in reflowable books to the paginator', async () => {
    const view = makeView()
    await renderReader(view)
    const doc = loadSection(view)
    fireEvent.touchStart(doc.body, { changedTouches: [{ screenX: 200, screenY: 100 }] })
    fireEvent.touchEnd(doc.body, { changedTouches: [{ screenX: 100, screenY: 105 }] })
    expect(view.goRight).not.toHaveBeenCalled()
  })

  it('navigates from the table of contents', async () => {
    const view = makeView({
      toc: [
        {
          label: 'Part One',
          href: 'p1.xhtml',
          subitems: [{ label: 'Chapter 1', href: 'c1.xhtml' }]
        },
        { label: 'Part Two', href: 'p2.xhtml' }
      ]
    })
    await renderReader(view)
    fireEvent.click(screen.getByRole('button', { name: 'Contents' }))
    const drawer = await screen.findByRole('dialog', { name: 'Contents' })
    fireEvent.click(within(drawer).getByRole('button', { name: 'Chapter 1' }))
    expect(view.goTo).toHaveBeenCalledWith('c1.xhtml')
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: 'Contents' })).not.toBeInTheDocument()
    )
  })

  it('ignores taps that select text or were already handled', async () => {
    const view = makeView()
    await renderReader(view)
    const doc = loadSection(view)
    doc.body.textContent = 'Some selectable words'
    doc.getSelection()?.selectAllChildren(doc.body)
    fireEvent.click(doc.body, { clientX: 290 })
    doc.getSelection()?.removeAllRanges()
    doc.body.addEventListener('click', (e) => e.preventDefault(), { capture: true })
    fireEvent.click(doc.body, { clientX: 290 })
    expect(view.goRight).not.toHaveBeenCalled()
  })

  it('turns pages on taps in the margins around a section', async () => {
    const view = makeView()
    await renderReader(view)
    fireEvent.click(view, { clientX: 290 })
    fireEvent.click(view, { clientX: 150 })
    expect(view.goRight).toHaveBeenCalledTimes(1)
  })

  it('turns pages with the arrow keys inside a section', async () => {
    const view = makeView()
    await renderReader(view)
    const doc = loadSection(view)
    fireEvent.keyDown(doc, { key: 'ArrowLeft' })
    fireEvent.keyDown(doc, { key: 'Enter' })
    expect(view.goLeft).toHaveBeenCalledTimes(1)
    expect(view.goRight).not.toHaveBeenCalled()
  })

  it('ignores vertical or incomplete swipes on PDFs', async () => {
    const view = makeView({ fixedLayout: true })
    await renderReader(view)
    const doc = loadSection(view)
    fireEvent.touchEnd(doc.body, { changedTouches: [{ screenX: 100, screenY: 100 }] })
    fireEvent.touchStart(doc.body, { changedTouches: [{ screenX: 100, screenY: 100 }] })
    fireEvent.touchEnd(doc.body, { changedTouches: [{ screenX: 110, screenY: 300 }] })
    fireEvent.touchStart(doc.body, { changedTouches: [] })
    fireEvent.touchEnd(doc.body, { changedTouches: [] })
    expect(view.goLeft).not.toHaveBeenCalled()
    expect(view.goRight).not.toHaveBeenCalled()
  })

  it('treats a relocate without progress as the start', async () => {
    const view = makeView()
    const onRelocate = jest.fn()
    await renderReader(view, { onRelocate })
    relocate(view, {})
    expect(onRelocate).toHaveBeenCalledWith({
      fraction: 0,
      section: 0,
      cfi: undefined,
      tocLabel: undefined,
      tocHref: undefined
    })
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
  })

  it('drops the view when unmounted before the book opens', async () => {
    const view = makeView()
    let resolveOpen = () => {}
    view.open.mockImplementation(() => new Promise<void>((r) => (resolveOpen = r)))
    mockCreateFoliateView.mockResolvedValue(view)
    const { unmount } = render(<BookReader url={URL_} title="Dune" onClose={jest.fn()} />)
    await waitFor(() => expect(view.open).toHaveBeenCalled())
    unmount()
    await act(async () => resolveOpen())
    expect(view.init).not.toHaveBeenCalled()
    expect(view.close).toHaveBeenCalled()
  })

  it('does not mount a view that loads after unmount', async () => {
    const view = makeView()
    let resolveView: (v: FakeView) => void = () => {}
    mockCreateFoliateView.mockImplementation(() => new Promise((r) => (resolveView = r)))
    const { unmount } = render(<BookReader url={URL_} title="Dune" onClose={jest.fn()} />)
    unmount()
    await act(async () => resolveView(view))
    expect(view.open).not.toHaveBeenCalled()
  })

  it('marks the current chapter in the table of contents', async () => {
    const view = makeView({
      toc: [
        { label: 'Notes', href: 'c1.xhtml', subitems: [] },
        { label: 'Notes', href: 'c2.xhtml' }
      ]
    })
    await renderReader(view)
    relocate(view, { fraction: 0.5, tocItem: { label: 'Notes', href: 'c2.xhtml' } })
    fireEvent.click(screen.getByRole('button', { name: 'Contents' }))
    const drawer = await screen.findByRole('dialog', { name: 'Contents' })
    const [first, second] = within(drawer).getAllByRole('button', { name: 'Notes' })
    expect(second).toHaveAttribute('aria-current', 'location')
    expect(first).not.toHaveAttribute('aria-current')
  })

  it('leaves the page alone on arrow keys inside an open dialog', async () => {
    const view = makeView({ toc: [{ label: 'Chapter 1', href: 'c1.xhtml' }] })
    await renderReader(view)
    fireEvent.click(screen.getByRole('button', { name: 'Contents' }))
    const drawer = await screen.findByRole('dialog', { name: 'Contents' })
    fireEvent.keyDown(within(drawer).getByRole('button', { name: 'Chapter 1' }), {
      key: 'ArrowRight'
    })
    expect(view.goRight).not.toHaveBeenCalled()
  })

  it('hides the contents control for a book without a TOC', async () => {
    await renderReader(makeView())
    expect(screen.queryByRole('button', { name: 'Contents' })).not.toBeInTheDocument()
  })

  it('applies the theme and font size to reflowable books', async () => {
    const view = makeView()
    await renderReader(view)
    expect(view.renderer.setStyles).toHaveBeenLastCalledWith(
      expect.stringContaining('font-size: 100%')
    )

    fireEvent.click(screen.getByRole('button', { name: 'Reading settings' }))
    const sheet = await screen.findByRole('dialog', { name: 'Reading settings' })
    fireEvent.click(within(sheet).getByRole('tab', { name: 'Sepia' }))
    fireEvent.click(within(sheet).getByRole('button', { name: 'Larger text' }))

    const css = String(view.renderer.setStyles.mock.calls.at(-1)?.[0])
    expect(css).toContain('font-size: 110%')
    expect(css).toContain('color-scheme: light')
    expect(within(sheet).getByText('110%')).toBeInTheDocument()

    fireEvent.click(within(sheet).getByRole('button', { name: 'Smaller text' }))
    expect(view.renderer.setStyles).toHaveBeenLastCalledWith(
      expect.stringContaining('font-size: 100%')
    )
  })

  it('tints PDFs instead of restyling them, without font controls', async () => {
    const view = makeView({ fixedLayout: true })
    await renderReader(view)
    fireEvent.click(screen.getByRole('button', { name: 'Reading settings' }))
    const sheet = await screen.findByRole('dialog', { name: 'Reading settings' })
    expect(within(sheet).queryByRole('button', { name: 'Larger text' })).not.toBeInTheDocument()
    fireEvent.click(within(sheet).getByRole('tab', { name: 'Dark' }))
    expect(view.style.filter).toContain('invert')
    expect(view.renderer.setStyles).not.toHaveBeenCalled()
  })
})

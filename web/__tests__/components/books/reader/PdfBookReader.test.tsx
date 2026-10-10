import React from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'

const mockOpenPdf = jest.fn()
const mockRenderPage = jest.fn()
const mockOutline = jest.fn()

jest.mock('@/lib/books/pdf', () => {
  const actual = jest.requireActual('@/lib/books/pdf')
  return {
    ...actual,
    openPdf: (...a: unknown[]) => mockOpenPdf(...a),
    renderPage: (...a: unknown[]) => mockRenderPage(...a),
    pdfOutline: (...a: unknown[]) => mockOutline(...a)
  }
})

class MockResizeObserver {
  constructor(private cb: (entries: unknown[]) => void) {}
  observe() {
    this.cb([{ contentRect: { width: 300, height: 600 } }])
  }
  disconnect() {}
}
// eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- jsdom has no ResizeObserver
global.ResizeObserver = MockResizeObserver as never

import PdfBookReader from '@/components/books/reader/PdfBookReader'

const destroy = jest.fn(async () => {})
const BOOK = new Blob(['%PDF'])

async function renderReader(props: Partial<React.ComponentProps<typeof PdfBookReader>> = {}) {
  const onRelocate = jest.fn()
  const onClose = jest.fn()
  render(
    <PdfBookReader file={BOOK} title="Scans" onClose={onClose} onRelocate={onRelocate} {...props} />
  )
  await screen.findByRole('button', { name: 'Reading settings' })
  return { onRelocate, onClose }
}

const surface = () => screen.getByTestId('pdf-surface')
const valuenow = () => screen.getByRole('progressbar').getAttribute('aria-valuenow')

beforeEach(() => {
  jest.clearAllMocks()
  localStorage.clear()
  mockOpenPdf.mockResolvedValue({ pdf: { numPages: 5 }, destroy })
  mockOutline.mockResolvedValue([
    { label: 'Intro', href: '#page=1' },
    { label: 'End', href: '#page=4' }
  ])
  mockRenderPage.mockResolvedValue(true)
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only left/width are read
  Element.prototype.getBoundingClientRect = () => ({ left: 0, width: 300 }) as DOMRect
})

describe('PdfBookReader', () => {
  it('opens at the first page and renders it to the available size', async () => {
    await renderReader()
    expect(screen.getByText('Scans')).toBeInTheDocument()
    expect(screen.getByText('Intro')).toBeInTheDocument()
    expect(valuenow()).toBe('0')
    await waitFor(() => expect(mockRenderPage).toHaveBeenCalled())
    expect(mockRenderPage.mock.calls[0]!.slice(1, 2)).toEqual([1])
    expect(mockRenderPage.mock.calls[0]!.slice(3, 5)).toEqual([300, 600])
  })

  it('opens at the stored page, else the stored percent', async () => {
    await renderReader({ initialPosition: { position: { page: 3 }, percent: 10 } })
    expect(valuenow()).toBe('50')
  })

  it('falls back to the percent when no page is stored', async () => {
    await renderReader({ initialPosition: { percent: 75 } })
    expect(valuenow()).toBe('75')
  })

  it('turns pages by tap, key and swipe and reports each turn', async () => {
    const { onRelocate } = await renderReader({
      initialPosition: { position: { page: 2 }, percent: 0 }
    })
    fireEvent.click(surface(), { clientX: 290 })
    expect(onRelocate).toHaveBeenLastCalledWith({
      fraction: 0.5,
      section: 2,
      tocLabel: 'Intro',
      position: { page: 3 }
    })
    fireEvent.click(surface(), { clientX: 150 })
    expect(onRelocate).toHaveBeenCalledTimes(1)
    fireEvent.click(surface(), { clientX: 10 })
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(onRelocate).toHaveBeenLastCalledWith(expect.objectContaining({ position: { page: 3 } }))
    fireEvent.touchStart(surface(), { changedTouches: [{ screenX: 200, screenY: 0 }] })
    fireEvent.touchEnd(surface(), { changedTouches: [{ screenX: 100, screenY: 0 }] })
    expect(onRelocate).toHaveBeenLastCalledWith(expect.objectContaining({ position: { page: 4 } }))
    fireEvent.touchEnd(surface(), { changedTouches: [{ screenX: 0, screenY: 0 }] })
    expect(onRelocate).toHaveBeenCalledTimes(4)
  })

  it('does not turn past either end or while a dialog is open', async () => {
    const { onRelocate } = await renderReader()
    fireEvent.keyDown(window, { key: 'ArrowLeft' })
    expect(onRelocate).not.toHaveBeenCalled()
    fireEvent.click(await screen.findByRole('button', { name: 'Contents' }))
    fireEvent.keyDown(await screen.findByRole('dialog'), { key: 'ArrowRight' })
    expect(onRelocate).not.toHaveBeenCalled()
  })

  it('jumps to a contents entry', async () => {
    const { onRelocate } = await renderReader()
    fireEvent.click(await screen.findByRole('button', { name: 'Contents' }))
    fireEvent.click(await screen.findByRole('button', { name: 'End' }))
    expect(onRelocate).toHaveBeenCalledWith(expect.objectContaining({ position: { page: 4 } }))
  })

  it('shows an error when the file cannot be opened or a page fails to render', async () => {
    mockOpenPdf.mockRejectedValueOnce(new Error('bad'))
    render(<PdfBookReader file={BOOK} title="Scans" onClose={jest.fn()} />)
    expect(await screen.findByText(/couldn.t load|failed|error/i)).toBeInTheDocument()
  })

  it('shows an error when rendering a page fails', async () => {
    mockRenderPage.mockRejectedValue(new Error('x'))
    render(<PdfBookReader file={BOOK} title="Scans" onClose={jest.fn()} />)
    expect(await screen.findByText(/couldn.t load|failed|error/i)).toBeInTheDocument()
  })

  it('closes, and releases the document on unmount', async () => {
    const onClose = jest.fn()
    const view = render(<PdfBookReader file={BOOK} title="Scans" onClose={onClose} />)
    await screen.findByRole('button', { name: 'Reading settings' })
    fireEvent.click(screen.getByRole('button', { name: 'Close reader' }))
    expect(onClose).toHaveBeenCalled()
    view.unmount()
    expect(destroy).toHaveBeenCalled()
  })

  it('releases a document that finished opening after unmount', async () => {
    let resolve: (v: unknown) => void = () => {}
    mockOpenPdf.mockReturnValue(new Promise((r) => (resolve = r)))
    const view = render(<PdfBookReader file={BOOK} title="Scans" onClose={jest.fn()} />)
    view.unmount()
    await act(async () => resolve({ pdf: { numPages: 1 }, destroy }))
    expect(destroy).toHaveBeenCalled()
  })
})

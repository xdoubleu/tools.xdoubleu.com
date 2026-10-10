import React from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'

type TimelineItem = { references: string[] }
type Listeners = {
  positionChanged: (locator: unknown) => void
  timelineItemChanged: (item: TimelineItem | undefined) => void
  tap: (e: { x: number; y: number }) => boolean
}

const mockLoadPublication = jest.fn()
const mockResumeLocator = jest.fn()
const mockLocatorToPosition = jest.fn()
let nav: FakeNavigator | null = null
let loadError: Error | null = null
let loadHangs = false

class FakeNavigator {
  listeners: Listeners
  initial: unknown
  load = jest.fn(async () => {
    if (loadError) throw loadError
    if (loadHangs) await new Promise(() => {})
  })
  destroy = jest.fn(async () => {})
  submitPreferences = jest.fn(async () => {})
  goBackward = jest.fn()
  goForward = jest.fn()
  goLink = jest.fn()
  timeline = {
    tocEntryFor: (item: TimelineItem) => {
      const href = item.references[0]
      const title = { 'res/ch1.xhtml#top': 'Chapter 1', 'res/part.xhtml': 'Part' }[href!]
      return title ? { link: { href, title } } : undefined
    }
  }
  constructor(_c: HTMLElement, _p: unknown, listeners: Listeners, _pos: unknown, initial: unknown) {
    this.listeners = listeners
    this.initial = initial
    // eslint-disable-next-line @typescript-eslint/no-this-alias -- exposes the instance to tests
    nav = this
  }
}

jest.mock(
  '@readium/navigator',
  () => ({
    EpubNavigator: function (...args: ConstructorParameters<typeof FakeNavigator>) {
      return new FakeNavigator(...args)
    },
    EpubPreferences: class {
      constructor(public prefs: unknown) {}
    }
  }),
  { virtual: true }
)
jest.mock(
  '@readium/shared',
  () => ({
    Link: class {
      constructor(public values: unknown) {}
    }
  }),
  { virtual: true }
)
jest.mock('@/lib/books/readium', () => ({
  loadPublication: (...a: unknown[]) => mockLoadPublication(...a),
  resumeLocator: (...a: unknown[]) => mockResumeLocator(...a),
  locatorToPosition: (...a: unknown[]) => mockLocatorToPosition(...a),
  overallFraction: () => 0.25,
  sectionIndex: () => 1,
  tocItems: (items: unknown[] | undefined) => items ?? [],
  readiumPreferences: (theme: string, size: number) => ({ theme, size })
}))

import ReadiumBookReader, {
  READIUM_LOAD_TIMEOUT_MS
} from '@/components/books/reader/ReadiumBookReader'

const positions = [{ href: 'res/ch2.xhtml', locations: { position: 1 } }]

const toc = [
  {
    label: 'Part',
    href: 'res/part.xhtml',
    subitems: [{ label: 'Chapter 1', href: 'res/ch1.xhtml#top' }]
  },
  { label: 'Group' }
]

async function renderReader(props: Partial<React.ComponentProps<typeof ReadiumBookReader>> = {}) {
  const onClose = jest.fn()
  const onRelocate = jest.fn()
  render(
    <ReadiumBookReader
      bookId="b1"
      title="Dune"
      onClose={onClose}
      onRelocate={onRelocate}
      {...props}
    />
  )
  await waitFor(() => expect(nav?.load).toHaveBeenCalled())
  return { onClose, onRelocate }
}

beforeEach(() => {
  nav = null
  loadError = null
  loadHangs = false
  localStorage.clear()
  mockLoadPublication.mockResolvedValue({
    manifest: { toc: { items: toc } },
    positionsFromManifest: () => Promise.resolve(positions)
  })
  mockResumeLocator.mockResolvedValue({ href: 'res/ch2.xhtml' })
  mockLocatorToPosition.mockResolvedValue({ href: 'ch2.xhtml', offset: 7 })
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only left/width are read
  Element.prototype.getBoundingClientRect = () => ({ left: 0, width: 300 }) as DOMRect
})

describe('ReadiumBookReader', () => {
  it('opens at the resume locator and shows the title and controls', async () => {
    await renderReader({ initialPosition: { percent: 40 } })
    expect(mockLoadPublication).toHaveBeenCalledWith('b1')
    expect(mockResumeLocator).toHaveBeenCalledWith(expect.anything(), { percent: 40 })
    expect(nav?.initial).toEqual({ href: 'res/ch2.xhtml' })
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Contents' })).toBeInTheDocument()
  })

  it('reports page turns with the neutral position, but not the opening location', async () => {
    const { onRelocate } = await renderReader()
    await screen.findByRole('button', { name: 'Contents' })
    act(() => nav!.listeners.positionChanged({ href: 'res/ch2.xhtml' }))
    await waitFor(() => expect(onRelocate).toHaveBeenCalledTimes(1))
    expect(onRelocate).toHaveBeenCalledWith(
      expect.objectContaining({
        fraction: 0.25,
        section: 1,
        position: { href: 'ch2.xhtml', offset: 7 }
      })
    )
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25')
  })

  it('turns pages on edge taps and arrow keys', async () => {
    await renderReader()
    await screen.findByRole('button', { name: 'Contents' })
    expect(nav!.listeners.tap({ x: 10, y: 0 })).toBe(true)
    expect(nav!.goBackward).toHaveBeenCalledTimes(1)
    expect(nav!.listeners.tap({ x: 290, y: 0 })).toBe(true)
    expect(nav!.goForward).toHaveBeenCalledTimes(1)
    expect(nav!.listeners.tap({ x: 150, y: 0 })).toBe(false)

    fireEvent.keyDown(window, { key: 'ArrowLeft' })
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(nav!.goBackward).toHaveBeenCalledTimes(2)
    expect(nav!.goForward).toHaveBeenCalledTimes(2)
  })

  it('navigates from the contents drawer', async () => {
    await renderReader()
    fireEvent.click(await screen.findByRole('button', { name: 'Contents' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Chapter 1' }))
    expect(nav!.goLink).toHaveBeenCalledWith(
      expect.objectContaining({ values: { href: 'res/ch1.xhtml#top' } }),
      false,
      expect.any(Function)
    )
  })

  it('applies reader settings as navigator preferences', async () => {
    await renderReader()
    await screen.findByRole('button', { name: 'Contents' })
    await waitFor(() => expect(nav!.submitPreferences).toHaveBeenCalled())
  })

  it('shows an error when the book fails to open', async () => {
    loadError = new Error('boom')
    render(<ReadiumBookReader bookId="b1" title="Dune" onClose={jest.fn()} />)
    expect(await screen.findByText(/couldn.t load|failed|error/i)).toBeInTheDocument()
  })

  it('shows an error instead of spinning when the navigator never finishes loading', async () => {
    loadHangs = true
    jest.useFakeTimers()
    try {
      render(<ReadiumBookReader bookId="b1" title="Dune" onClose={jest.fn()} />)
      await waitFor(() => expect(nav?.load).toHaveBeenCalled())
      expect(screen.getByText('Loading book…')).toBeInTheDocument()
      await act(async () => {
        jest.advanceTimersByTime(READIUM_LOAD_TIMEOUT_MS)
      })
      expect(screen.getByText(/couldn.t load|failed|error/i)).toBeInTheDocument()
      expect(screen.queryByText('Loading book…')).not.toBeInTheDocument()
    } finally {
      jest.useRealTimers()
    }
  })

  it('shows an error instead of a blank page when the book has no positions', async () => {
    mockLoadPublication.mockResolvedValue({
      manifest: {},
      positionsFromManifest: () => Promise.resolve([])
    })
    render(<ReadiumBookReader bookId="b1" title="Dune" onClose={jest.fn()} />)
    expect(await screen.findByText(/couldn.t load|failed|error/i)).toBeInTheDocument()
    expect(nav).toBeNull()
  })

  it('closes and destroys the navigator on unmount', async () => {
    const onClose = jest.fn()
    const { unmount } = render(<ReadiumBookReader bookId="b1" title="Dune" onClose={onClose} />)
    await waitFor(() => expect(nav?.load).toHaveBeenCalled())
    fireEvent.click(screen.getByRole('button', { name: 'Close reader' }))
    expect(onClose).toHaveBeenCalled()
    unmount()
    expect(nav!.destroy).toHaveBeenCalled()
  })

  it('labels the section Readium reports on screen and highlights it in the contents', async () => {
    await renderReader()
    await screen.findByRole('button', { name: 'Contents' })
    act(() => nav!.listeners.timelineItemChanged({ references: ['res/part.xhtml'] }))
    expect(await screen.findByText('Part')).toBeInTheDocument()
    act(() => nav!.listeners.timelineItemChanged({ references: ['res/ch1.xhtml#top'] }))
    expect(await screen.findByText('Chapter 1')).toBeInTheDocument()
    expect(screen.queryByText('Part')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Contents' }))
    expect(await screen.findByRole('button', { name: 'Chapter 1' })).toHaveAttribute(
      'aria-current',
      'location'
    )
  })

  it('clears the section label when no contents entry covers the page', async () => {
    await renderReader()
    await screen.findByRole('button', { name: 'Contents' })
    act(() => nav!.listeners.timelineItemChanged({ references: ['res/ch1.xhtml#top'] }))
    expect(await screen.findByText('Chapter 1')).toBeInTheDocument()
    act(() => nav!.listeners.timelineItemChanged({ references: ['res/other.xhtml'] }))
    await waitFor(() => expect(screen.queryByText('Chapter 1')).not.toBeInTheDocument())
    act(() => nav!.listeners.timelineItemChanged(undefined))
    expect(screen.queryByText('Chapter 1')).not.toBeInTheDocument()
  })

  it('ignores arrow keys while a dialog is open', async () => {
    await renderReader()
    fireEvent.click(await screen.findByRole('button', { name: 'Contents' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.keyDown(dialog, { key: 'ArrowRight' })
    expect(nav!.goForward).not.toHaveBeenCalled()
  })

  it('ignores page changes before the book has opened', async () => {
    const { onRelocate } = await renderReader()
    act(() => nav!.listeners.positionChanged({ href: 'res/ch2.xhtml' }))
    expect(onRelocate).not.toHaveBeenCalled()
  })

  it('does not build a navigator when unmounted while loading', async () => {
    let resolve: (pub: unknown) => void = () => {}
    mockLoadPublication.mockReturnValue(new Promise((r) => (resolve = r)))
    const { unmount } = render(<ReadiumBookReader bookId="b1" title="Dune" onClose={jest.fn()} />)
    unmount()
    await act(async () => {
      resolve({ manifest: {}, positionsFromManifest: () => Promise.resolve(positions) })
    })
    expect(nav).toBeNull()
  })
})

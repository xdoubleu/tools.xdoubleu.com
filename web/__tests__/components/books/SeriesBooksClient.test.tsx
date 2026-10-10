import React from 'react'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { GetSeriesResponseSchema } from '@/lib/gen/books/v1/library_pb'

const mockAddBook = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useBookSeries', () => ({ useBookSeries: jest.fn() }))
jest.mock('@/hooks/useBooks', () => ({ useCreateBook: () => mockAddBook }))
jest.mock('swr', () => ({ mutate: (...args: unknown[]) => mockMutate(...args) }))

jest.mock('next/link', () => {
  const Link = ({
    children,
    href,
    'aria-label': ariaLabel
  }: {
    children: React.ReactNode
    href: string
    'aria-label'?: string
  }) => (
    <a href={href} aria-label={ariaLabel}>
      {children}
    </a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

jest.mock('next/image', () => {
  return function MockImage({ src, alt }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} />
  }
})

import SeriesBooksClient from '@/components/books/SeriesBooksClient'
import { useBookSeries } from '@/hooks/useBookSeries'

const mockUseBookSeries = jest.mocked(useBookSeries)

function series(overrides = {}) {
  return create(GetSeriesResponseSchema, {
    name: 'Discworld',
    total: 41,
    entries: [
      {
        position: 1,
        userBook: { id: 'ub-1', status: 'read', book: { title: 'The Colour of Magic' } }
      },
      {
        position: 2,
        external: {
          provider: 'hardcover',
          title: 'The Light Fantastic',
          authors: ['Terry Pratchett', 'Co Author'],
          seriesName: 'Discworld',
          seriesPosition: 2,
          seriesTotal: 41
        }
      },
      { userBook: { id: 'ub-3', status: 'to-read', book: { title: 'Unordered' } } }
    ],
    ...overrides
  })
}

function mockState(state: Partial<ReturnType<typeof useBookSeries>>) {
  // @ts-expect-error -- partial SWRResponse for test purposes
  mockUseBookSeries.mockReturnValue({
    data: undefined,
    error: undefined,
    isLoading: false,
    ...state
  })
}

describe('SeriesBooksClient', () => {
  beforeEach(() => {
    mockAddBook.mockReset()
    mockMutate.mockReset()
  })

  it('lists owned and missing volumes in order', () => {
    mockState({ data: series() })
    render(<SeriesBooksClient name="Discworld" />)

    expect(mockUseBookSeries).toHaveBeenCalledWith('Discworld')
    expect(screen.getByRole('link', { name: 'Books' })).toHaveAttribute(
      'href',
      '/dashboard/reading'
    )
    expect(screen.getByRole('link', { name: 'Library' })).toHaveAttribute('href', '/books/library')
    expect(within(screen.getByRole('navigation')).getByText('Discworld')).toBeInTheDocument()
    expect(screen.queryByText('No books in this series yet.')).toBeNull()
    expect(screen.getByText('1 of 41 read · 2 in your library · 1 missing')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'The Colour of Magic' })).toHaveAttribute(
      'href',
      '/books/ub-1'
    )
    expect(screen.getByText('#1').tagName).toBe('P')
    expect(screen.getByText('#2').tagName).toBe('P')
    expect(screen.getByText('Read')).toHaveClass('text-success')
    expect(screen.getByText('Want to read')).toHaveClass('text-subtle')
    expect(screen.getByTestId('series-missing')).toHaveTextContent('The Light Fantastic')
    expect(screen.getByText('Terry Pratchett, Co Author')).toBeInTheDocument()
    expect(screen.queryByText('Failed to add. Try again.')).toBeNull()
    const unordered = screen.getByRole('link', { name: 'Unordered' })
    expect(unordered.querySelectorAll('p')).toHaveLength(0)
  })

  it('adds a missing volume to the to-read shelf with its series', async () => {
    mockAddBook.mockResolvedValue({})
    mockState({ data: series() })
    render(<SeriesBooksClient name="Discworld" />)

    fireEvent.click(screen.getByRole('button', { name: 'Add to Want to read' }))
    await waitFor(() =>
      expect(mockAddBook).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'The Light Fantastic',
          author: 'Terry Pratchett, Co Author',
          status: 'to-read',
          seriesName: 'Discworld',
          seriesPosition: 2,
          seriesTotal: 41
        })
      )
    )
    await waitFor(() => expect(mockMutate).toHaveBeenCalledWith(['/books/series', 'Discworld']))
    expect(mockMutate).toHaveBeenCalledWith('/books')
  })

  it('shows a pending add, then a failure that a retry clears', async () => {
    let reject: (e: Error) => void = () => {}
    mockAddBook.mockReturnValueOnce(new Promise((_, r) => (reject = r)))
    mockState({ data: series() })
    render(<SeriesBooksClient name="Discworld" />)

    fireEvent.click(screen.getByRole('button', { name: 'Add to Want to read' }))
    expect(screen.getByRole('button', { name: 'Adding…' })).toBeDisabled()
    reject(new Error('nope'))
    expect(await screen.findByText('Failed to add. Try again.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add to Want to read' })).toBeEnabled()

    mockAddBook.mockResolvedValueOnce({})
    fireEvent.click(screen.getByRole('button', { name: 'Add to Want to read' }))
    await waitFor(() => expect(screen.queryByText('Failed to add. Try again.')).toBeNull())
  })

  it('omits the position and authors lines when a missing volume has none', () => {
    mockState({
      data: series({ entries: [{ external: { provider: 'hardcover', title: 'Bare' } }] })
    })
    render(<SeriesBooksClient name="Discworld" />)
    expect(screen.getByTestId('series-missing').querySelectorAll('p')).toHaveLength(0)
  })

  it('says when Hardcover was unavailable', () => {
    mockState({ data: series({ externalUnavailable: true }) })
    render(<SeriesBooksClient name="Discworld" />)
    expect(screen.getByText(/Couldn.t check Hardcover/)).toBeInTheDocument()
  })

  it('renders loading, error and empty states', () => {
    mockState({ isLoading: true })
    const { rerender } = render(<SeriesBooksClient name="Discworld" />)
    expect(screen.getByText('Loading series…')).toBeInTheDocument()
    expect(screen.queryByText('No books in this series yet.')).toBeNull()

    mockState({ error: new Error('x') })
    rerender(<SeriesBooksClient name="Discworld" />)
    expect(screen.getByText('Failed to load series.')).toBeInTheDocument()

    mockState({ data: series({ entries: [] }) })
    rerender(<SeriesBooksClient name="Discworld" />)
    expect(screen.getByText('No books in this series yet.')).toBeInTheDocument()
    expect(screen.queryByTestId('series-entries')).toBeNull()
  })

  it('skips entries with neither side set', () => {
    mockState({ data: series({ entries: [{ position: 1 }] }) })
    render(<SeriesBooksClient name="Discworld" />)
    screen
      .getByRole('list')
      .querySelectorAll('li')
      .forEach((li) => expect(li).toBeEmptyDOMElement())
    expect(screen.queryByTestId('series-missing')).toBeNull()
  })
})

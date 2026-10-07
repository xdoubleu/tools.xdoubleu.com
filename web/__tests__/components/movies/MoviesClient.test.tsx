import React from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockAdd = jest.fn()
const mockSetStatus = jest.fn()
const mockFetchPage = jest.fn()

jest.mock('@/hooks/useMovies', () => ({
  useMoviesBacklog: jest.fn(),
  useFetchMoviesBacklogPage: jest.fn(() => mockFetchPage),
  useMovieSearch: jest.fn(),
  useMoviesActions: jest.fn(() => ({
    add: mockAdd,
    setStatus: mockSetStatus,
    remove: jest.fn()
  }))
}))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

jest.mock('next/image', () => ({
  __esModule: true,
  default: ({ src, onError }: { src: string; onError: () => void }) => (
    // eslint-disable-next-line @next/next/no-img-element -- next/image stand-in
    <img src={src} alt="" data-testid="poster" onError={onError} />
  )
}))

import MoviesClient from '@/components/movies/MoviesClient'
import { useMovieSearch, useMoviesBacklog } from '@/hooks/useMovies'
import {
  BacklogEntrySchema,
  ListBacklogResponseSchema,
  SearchResultSchema,
  SearchTitlesResponseSchema
} from '@/lib/gen/movies/v1/movies_pb'

const matrix = create(BacklogEntrySchema, {
  id: 'e-1',
  mediaType: 'movie',
  tmdbId: 603n,
  title: 'The Matrix',
  releaseDate: '1999-03-30',
  posterPath: '/matrix.jpg',
  status: 'want'
})
const avatar = create(BacklogEntrySchema, {
  id: 'e-2',
  mediaType: 'movie',
  tmdbId: 999n,
  title: 'Avatar 5',
  releaseDate: '2099-12-18',
  status: 'want'
})

function mockBacklog(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useMoviesBacklog).mockReturnValue({ isLoading: false, ...value })
}

function mockSearch(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useMovieSearch).mockReturnValue({ isLoading: false, ...value })
}

function search(text: string) {
  fireEvent.change(screen.getByLabelText('Search TMDB'), { target: { value: text } })
  act(() => {
    jest.advanceTimersByTime(300)
  })
}

beforeEach(() => {
  jest.clearAllMocks()
  mockSearch({})
})

describe('MoviesClient backlog', () => {
  it('lists entries with poster, type and release info', () => {
    mockBacklog({
      data: create(ListBacklogResponseSchema, { entries: [matrix, avatar], hasMore: false })
    })
    render(<MoviesClient />)

    expect(screen.getByRole('link', { name: /The Matrix/ })).toHaveAttribute('href', '/movies/e-1')
    expect(screen.getByText(/1999/)).toBeInTheDocument()
    expect(screen.getByText(/Releases 2099-12-18/)).toBeInTheDocument()
    expect(screen.getByTestId('poster')).toHaveAttribute(
      'src',
      'https://image.tmdb.org/t/p/w92/matrix.jpg'
    )
    expect(screen.getByText(/not endorsed or certified by TMDB/)).toBeInTheDocument()
  })

  it('falls back to an initial when the poster fails to load', () => {
    mockBacklog({ data: create(ListBacklogResponseSchema, { entries: [matrix] }) })
    render(<MoviesClient />)
    fireEvent.error(screen.getByTestId('poster'))
    expect(screen.queryByTestId('poster')).not.toBeInTheDocument()
    expect(screen.getByText('T')).toBeInTheDocument()
  })

  it('changes status inline', async () => {
    mockSetStatus.mockResolvedValue(undefined)
    mockBacklog({ data: create(ListBacklogResponseSchema, { entries: [matrix] }) })
    render(<MoviesClient />)

    fireEvent.change(screen.getByLabelText('Status of The Matrix'), {
      target: { value: 'watched' }
    })
    await waitFor(() => expect(mockSetStatus).toHaveBeenCalledWith('e-1', 'watched'))
    // Held even though the (unrefetched) entry still says "want".
    await waitFor(() =>
      expect(screen.getByLabelText('Status of The Matrix')).toHaveValue('watched')
    )
  })

  it('reports a failed status change', async () => {
    mockSetStatus.mockRejectedValue(new Error('nope'))
    mockBacklog({ data: create(ListBacklogResponseSchema, { entries: [matrix] }) })
    render(<MoviesClient />)

    await act(async () =>
      fireEvent.change(screen.getByLabelText('Status of The Matrix'), {
        target: { value: 'watched' }
      })
    )
    expect(screen.getByText(/Couldn.t update status/)).toBeInTheDocument()
    expect(screen.getByLabelText('Status of The Matrix')).toHaveValue('want')
  })

  it('refetches with the chosen filters', () => {
    mockBacklog({ data: create(ListBacklogResponseSchema, { entries: [] }) })
    render(<MoviesClient />)

    fireEvent.click(screen.getByRole('tab', { name: 'Watched' }))
    fireEvent.click(screen.getByRole('tab', { name: 'Series' }))
    fireEvent.change(screen.getByLabelText('Sort'), { target: { value: 'title' } })

    expect(jest.mocked(useMoviesBacklog)).toHaveBeenLastCalledWith({
      status: 'watched',
      mediaType: 'series',
      sort: 'title'
    })
    expect(screen.getByText(/Nothing here yet/)).toBeInTheDocument()
  })

  it('loads more pages', async () => {
    mockFetchPage.mockResolvedValue({ items: [avatar], hasMore: false })
    mockBacklog({ data: create(ListBacklogResponseSchema, { entries: [matrix], hasMore: true }) })
    render(<MoviesClient />)

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Load more' })))
    expect(mockFetchPage).toHaveBeenCalledWith(1)
    expect(screen.getByText('Avatar 5')).toBeInTheDocument()
  })

  it('shows loading and error states', () => {
    mockBacklog({ isLoading: true })
    const { rerender } = render(<MoviesClient />)
    expect(screen.getByText('Loading backlog…')).toBeInTheDocument()

    mockBacklog({ error: new Error('boom') })
    rerender(<MoviesClient />)
    expect(screen.getByText('Failed to load backlog.')).toBeInTheDocument()
  })
})

describe('MoviesClient search', () => {
  const results = create(SearchTitlesResponseSchema, {
    results: [
      create(SearchResultSchema, {
        mediaType: 'series',
        tmdbId: 1396n,
        title: 'Breaking Bad',
        releaseDate: '2008-01-20'
      }),
      create(SearchResultSchema, {
        mediaType: 'movie',
        tmdbId: 603n,
        title: 'The Matrix',
        status: 'watched'
      })
    ]
  })

  beforeEach(() => {
    mockBacklog({})
    jest.useFakeTimers()
  })
  afterEach(() => jest.useRealTimers())

  it('waits for typing to pause before searching', () => {
    render(<MoviesClient />)
    fireEvent.change(screen.getByLabelText('Search TMDB'), { target: { value: 'br' } })
    expect(screen.getByRole('region', { name: 'Backlog' })).toBeInTheDocument()
    act(() => {
      jest.advanceTimersByTime(300)
    })
    expect(screen.queryByRole('region', { name: 'Backlog' })).not.toBeInTheDocument()
  })

  it('keeps the backlog for one-character queries', () => {
    render(<MoviesClient />)
    search('b')
    expect(screen.getByRole('region', { name: 'Backlog' })).toBeInTheDocument()
  })

  it('shows results with quick actions or the existing status', () => {
    mockSearch({ data: results })
    render(<MoviesClient />)
    search('br')

    expect(screen.getByText('Breaking Bad')).toBeInTheDocument()
    expect(screen.getByText('2008')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Want' })).toHaveLength(1)
    expect(screen.getByText('Watched', { selector: 'span' })).toBeInTheDocument()
  })

  it.each([
    ['Want', 'want'],
    ['Watched', 'watched']
  ])('adds with %s', async (label, status) => {
    mockAdd.mockResolvedValue({})
    mockSearch({ data: results })
    render(<MoviesClient />)
    search('br')

    await act(async () => fireEvent.click(screen.getByRole('button', { name: label })))
    expect(mockAdd).toHaveBeenCalledWith('series', 1396n, status)
  })

  it('reports a failed add', async () => {
    mockAdd.mockRejectedValue(new Error('nope'))
    mockSearch({ data: results })
    render(<MoviesClient />)
    search('br')

    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Want' })))
    expect(screen.getByText(/Couldn.t add/)).toBeInTheDocument()
  })

  it('shows loading, empty and error states', () => {
    mockSearch({ isLoading: true })
    const { rerender } = render(<MoviesClient />)
    search('zz')
    expect(screen.getByText('Loading results…')).toBeInTheDocument()

    mockSearch({ data: create(SearchTitlesResponseSchema, { results: [] }) })
    rerender(<MoviesClient />)
    expect(screen.getByText(/No movies or series match/)).toBeInTheDocument()

    mockSearch({ error: new ConnectError('off', Code.FailedPrecondition) })
    rerender(<MoviesClient />)
    expect(screen.getByText(/TMDB is not configured/)).toBeInTheDocument()

    mockSearch({ error: new Error('boom') })
    rerender(<MoviesClient />)
    expect(screen.getByText('Search failed. Try again.')).toBeInTheDocument()
  })
})

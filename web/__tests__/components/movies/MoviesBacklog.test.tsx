import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockSetStatus = jest.fn()
const mockFetchPage = jest.fn()

jest.mock('@/hooks/useMovies', () => ({
  useMoviesBacklog: jest.fn(),
  useFetchMoviesBacklogPage: jest.fn(() => mockFetchPage),
  useMoviesActions: jest.fn(() => ({ setStatus: mockSetStatus }))
}))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

import MoviesBacklog from '@/components/movies/MoviesBacklog'
import { useMoviesBacklog } from '@/hooks/useMovies'
import { BacklogEntrySchema, ListBacklogResponseSchema } from '@/lib/gen/movies/v1/movies_pb'

const entry = (id: string, title: string, releaseDate = '') =>
  create(BacklogEntrySchema, { id, mediaType: 'movie', title, releaseDate, status: 'want' })
const matrix = entry('e-1', 'The Matrix', '1999-03-30')
const avatar = entry('e-2', 'Avatar 5')
const dune = entry('e-3', 'Dune')

function mockBacklog(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useMoviesBacklog).mockReturnValue({ isLoading: false, ...value })
}

const page = (entries: ReturnType<typeof entry>[], hasMore = false) =>
  create(ListBacklogResponseSchema, { entries, hasMore })

function deferred() {
  let resolve!: () => void
  let reject!: (e: Error) => void
  const promise = new Promise<void>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

beforeEach(() => jest.clearAllMocks())

describe('MoviesBacklog filters', () => {
  it('offers every status and type tab and filters by them', () => {
    mockBacklog({ data: page([]) })
    render(<MoviesBacklog />)
    const last = () => jest.mocked(useMoviesBacklog).mock.lastCall?.[0]

    fireEvent.click(screen.getByRole('tab', { name: 'Watched' }))
    fireEvent.click(screen.getByRole('tab', { name: 'All statuses' }))
    expect(last()?.status).toBe('')
    for (const name of ['Want', 'Watching', 'Dropped']) {
      expect(screen.getByRole('tab', { name })).toBeInTheDocument()
    }

    fireEvent.click(screen.getByRole('tab', { name: 'Movies' }))
    expect(last()?.mediaType).toBe('movie')
    fireEvent.click(screen.getByRole('tab', { name: 'All types' }))
    expect(last()?.mediaType).toBe('')
  })

  it('offers New seasons right after Watched', () => {
    mockBacklog({ data: page([]) })
    render(<MoviesBacklog />)
    const tabs = screen.getAllByRole('tab').map((t) => t.textContent)
    expect(tabs.slice(0, 6)).toEqual([
      'All statuses',
      'Want',
      'Watching',
      'Watched',
      'New seasons',
      'Dropped'
    ])
    fireEvent.click(screen.getByRole('tab', { name: 'New seasons' }))
    expect(jest.mocked(useMoviesBacklog).mock.lastCall?.[0].status).toBe('new')
  })

  it('sorts by rating', () => {
    mockBacklog({ data: page([]) })
    render(<MoviesBacklog />)
    fireEvent.change(screen.getByLabelText('Sort'), { target: { value: 'rating' } })
    expect(jest.mocked(useMoviesBacklog).mock.lastCall?.[0].sort).toBe('rating')
    expect(screen.getByRole('option', { name: 'Rating' })).toBeInTheDocument()
  })
})

describe('MoviesBacklog rows', () => {
  it('shows type and release, or type alone when undated', () => {
    mockBacklog({ data: page([matrix, avatar]) })
    render(<MoviesBacklog />)
    const metas = screen.getAllByText(/^Movie/, { selector: 'p' })
    expect(metas.map((p) => p.textContent)).toEqual(['Movie · 1999', 'Movie'])
    expect(screen.getByLabelText('Status of The Matrix')).toHaveAttribute('id', 'status-e-1')
  })

  it('badges a series with a new season', () => {
    const caughtUp = create(BacklogEntrySchema, { ...matrix, hasNewSeason: false })
    const behind = create(BacklogEntrySchema, { ...avatar, hasNewSeason: true })
    mockBacklog({ data: page([caughtUp, behind]) })
    render(<MoviesBacklog />)
    const [first, second] = screen.getAllByRole('link')
    expect(first).not.toHaveTextContent('New season')
    expect(second).toHaveTextContent('New season')
  })

  it('shows a rating read-only, and nothing when unrated', () => {
    const rated = create(BacklogEntrySchema, { ...matrix, rating: 4 })
    mockBacklog({ data: page([rated, avatar]) })
    render(<MoviesBacklog />)
    expect(screen.getByRole('img', { name: '4 out of 5 stars' })).toBeInTheDocument()
    expect(screen.getAllByRole('img', { name: /stars|No rating/ })).toHaveLength(1)
    expect(screen.queryByRole('button', { name: /^Rate/ })).not.toBeInTheDocument()
  })

  it('disables the status while saving and clears an old error on success', async () => {
    mockBacklog({ data: page([matrix]) })
    render(<MoviesBacklog />)
    const select = () => screen.getByLabelText('Status of The Matrix')
    expect(select()).toBeEnabled()
    expect(screen.queryByText(/Couldn.t update status/)).not.toBeInTheDocument()

    mockSetStatus.mockRejectedValueOnce(new Error('nope'))
    await act(async () => fireEvent.change(select(), { target: { value: 'watched' } }))
    expect(screen.getByText(/Couldn.t update status/)).toBeInTheDocument()
    expect(select()).toBeEnabled()

    const save = deferred()
    mockSetStatus.mockReturnValueOnce(save.promise)
    fireEvent.change(select(), { target: { value: 'watched' } })
    expect(select()).toBeDisabled()
    expect(screen.queryByText(/Couldn.t update status/)).not.toBeInTheDocument()
    await act(async () => save.resolve())
    expect(select()).toBeEnabled()
  })
})

describe('MoviesBacklog states', () => {
  it('shows no loading or empty state alongside data', () => {
    mockBacklog({ data: page([matrix]), isLoading: true })
    render(<MoviesBacklog />)
    expect(screen.queryByText('Loading backlog…')).not.toBeInTheDocument()
    expect(screen.queryByText(/Nothing here yet/)).not.toBeInTheDocument()
  })

  it('shows no empty state before data arrives', () => {
    mockBacklog({})
    render(<MoviesBacklog />)
    expect(screen.queryByText(/Nothing here yet/)).not.toBeInTheDocument()
  })

  it('offers no load more for an empty page', () => {
    mockBacklog({ data: page([], true) })
    render(<MoviesBacklog />)
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  })

  it('follows new data from SWR', () => {
    mockBacklog({ data: page([matrix]) })
    const { rerender } = render(<MoviesBacklog />)
    mockBacklog({ data: page([avatar]) })
    rerender(<MoviesBacklog />)
    expect(screen.getByText('Avatar 5')).toBeInTheDocument()
    expect(screen.queryByText('The Matrix')).not.toBeInTheDocument()
  })
})

describe('MoviesBacklog pagination on revalidation', () => {
  async function loadSecondPage() {
    mockFetchPage.mockResolvedValue({ items: [dune], hasMore: false })
    mockBacklog({ data: page([matrix], true) })
    const view = render(<MoviesBacklog />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Load more' })))
    expect(screen.getByText('Dune')).toBeInTheDocument()
    return view
  }

  it('keeps loaded pages when the first page is unchanged', async () => {
    const { rerender } = await loadSecondPage()
    mockBacklog({ data: page([matrix], true) })
    rerender(<MoviesBacklog />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
  })

  it('drops loaded pages when the first page changed', async () => {
    const { rerender } = await loadSecondPage()
    mockBacklog({ data: page([avatar], true) })
    rerender(<MoviesBacklog />)
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
  })
})

describe('MoviesBacklog series status', () => {
  const series = (seasonCount: number) =>
    create(BacklogEntrySchema, {
      id: 's-1',
      mediaType: 'series',
      title: 'Dark',
      seasonCount,
      status: 'want'
    })

  it('asks when a multi-season series was watched', async () => {
    mockSetStatus.mockResolvedValue(undefined)
    mockBacklog({ data: page([series(2)]) })
    render(<MoviesBacklog />)

    fireEvent.change(screen.getByLabelText('Status of Dark'), { target: { value: 'watched' } })
    expect(mockSetStatus).not.toHaveBeenCalled()
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'A while ago' })))
    expect(mockSetStatus).toHaveBeenCalledWith('s-1', 'watched', true)
    expect(screen.queryByText('When did you watch Dark?')).not.toBeInTheDocument()
  })

  it.each([
    ['a single-season series', 1],
    ['a series without a known season count', 0]
  ])('marks %s watched today in one step', async (_name, seasonCount) => {
    mockSetStatus.mockResolvedValue(undefined)
    mockBacklog({ data: page([series(seasonCount)]) })
    render(<MoviesBacklog />)

    await act(async () =>
      fireEvent.change(screen.getByLabelText('Status of Dark'), { target: { value: 'watched' } })
    )
    expect(mockSetStatus).toHaveBeenCalledWith('s-1', 'watched', false)
  })

  it('asks only for watched', async () => {
    mockSetStatus.mockResolvedValue(undefined)
    mockBacklog({ data: page([series(2)]) })
    render(<MoviesBacklog />)

    await act(async () =>
      fireEvent.change(screen.getByLabelText('Status of Dark'), { target: { value: 'dropped' } })
    )
    expect(mockSetStatus).toHaveBeenCalledWith('s-1', 'dropped', false)
  })
})

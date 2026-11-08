import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockSetStatus = vi.fn()
const mockFetchPage = vi.fn()

vi.mock('@/hooks/useMovies', () => ({
  useMoviesBacklog: vi.fn(),
  useFetchMoviesBacklogPage: vi.fn(() => mockFetchPage),
  useMoviesActions: vi.fn(() => ({ setStatus: mockSetStatus }))
}))

import MoviesBacklog from '@/components/movies/MoviesBacklog'
import MoviesBacklogFilters from '@/components/movies/MoviesBacklogFilters'
import { DEFAULT_BACKLOG_FILTER } from '@/lib/movies/format'
import { useMoviesBacklog } from '@/hooks/useMovies'
import { BacklogEntrySchema, ListBacklogResponseSchema } from '@/lib/gen/movies/v1/movies_pb'

const entry = (id: string, title: string, releaseDate = '') =>
  create(BacklogEntrySchema, { id, mediaType: 'movie', title, releaseDate, status: 'want' })
const matrix = entry('e-1', 'The Matrix', '1999-03-30')
const avatar = entry('e-2', 'Avatar 5')
const dune = entry('e-3', 'Dune')

function mockBacklog(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useMoviesBacklog).mockReturnValue({ isLoading: false, ...value })
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

beforeEach(() => vi.clearAllMocks())

describe('MoviesBacklogFilters', () => {
  const renderFilters = () => {
    const onChange = vi.fn()
    render(<MoviesBacklogFilters filter={DEFAULT_BACKLOG_FILTER} onChange={onChange} />)
    return onChange
  }
  const options = (label: string) =>
    Array.from((screen.getByLabelText(label) as HTMLSelectElement).options).map((o) => o.text)

  it('offers every status, with New seasons right after Watched', () => {
    const onChange = renderFilters()
    expect(options('Status')).toEqual([
      'All statuses',
      'Want',
      'Watching',
      'Watched',
      'New seasons',
      'Dropped'
    ])
    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'new' } })
    expect(onChange).toHaveBeenLastCalledWith({ ...DEFAULT_BACKLOG_FILTER, status: 'new' })
  })

  it('filters by type and sorts', () => {
    const onChange = renderFilters()
    expect(options('Type')).toEqual(['All types', 'Movies', 'Series'])
    fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'movie' } })
    expect(onChange).toHaveBeenLastCalledWith({ ...DEFAULT_BACKLOG_FILTER, mediaType: 'movie' })
    fireEvent.change(screen.getByLabelText('Sort'), { target: { value: 'rating' } })
    expect(onChange).toHaveBeenLastCalledWith({ ...DEFAULT_BACKLOG_FILTER, sort: 'rating' })
  })

  it('toggles titles on my services', () => {
    const onChange = renderFilters()
    fireEvent.click(screen.getByRole('button', { name: 'On my services' }))
    expect(onChange).toHaveBeenLastCalledWith({ ...DEFAULT_BACKLOG_FILTER, onMyServices: true })
  })
})

describe('MoviesBacklog count', () => {
  it.each([
    [[matrix], false, '1 title'],
    [[matrix, avatar], false, '2 titles'],
    [[matrix], true, '1+ titles']
  ])('reports %#', (entries, hasMore, label) => {
    mockBacklog({ data: page(entries, hasMore) })
    const onCount = vi.fn()
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} onCount={onCount} />)
    expect(onCount).toHaveBeenLastCalledWith(label)
  })

  it('reports nothing before data arrives', () => {
    mockBacklog({})
    const onCount = vi.fn()
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} onCount={onCount} />)
    expect(onCount).toHaveBeenLastCalledWith(null)
  })
})

describe('MoviesBacklog rows', () => {
  it('shows type and release, or type alone when undated', () => {
    mockBacklog({ data: page([matrix, avatar]) })
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    const metas = screen.getAllByText(/^Movie/, { selector: 'p' })
    expect(metas.map((p) => p.textContent)).toEqual(['Movie · 1999', 'Movie'])
    expect(screen.getByLabelText('Status of The Matrix')).toHaveAttribute('id', 'status-e-1')
  })

  it('badges a series with a new season', () => {
    const caughtUp = create(BacklogEntrySchema, { ...matrix, hasNewSeason: false })
    const behind = create(BacklogEntrySchema, { ...avatar, hasNewSeason: true })
    mockBacklog({ data: page([caughtUp, behind]) })
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    const [first, second] = screen.getAllByRole('link')
    expect(first).not.toHaveTextContent('New season')
    expect(second).toHaveTextContent('New season')
    expect(screen.queryByRole('img', { name: 'No rating' })).not.toBeInTheDocument()
  })

  it('shows a rating read-only, and nothing when unrated', () => {
    const rated = create(BacklogEntrySchema, { ...matrix, rating: 4 })
    mockBacklog({ data: page([rated, avatar]) })
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    expect(screen.getByRole('img', { name: '4 out of 5 stars' })).toBeInTheDocument()
    expect(screen.getAllByRole('img', { name: /stars|No rating/ })).toHaveLength(1)
    expect(screen.queryByRole('button', { name: /^Rate/ })).not.toBeInTheDocument()
  })

  it('disables the status while saving and clears an old error on success', async () => {
    mockBacklog({ data: page([matrix]) })
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
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
  it('offers an empty-state action that focuses the search field', () => {
    mockBacklog({ data: page([]) })
    render(
      <>
        <input id="movies-search" aria-label="Search" />
        <MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />
      </>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Add a movie or series' }))
    expect(screen.getByLabelText('Search')).toHaveFocus()
  })

  it('shows no loading or empty state alongside data', () => {
    mockBacklog({ data: page([matrix]), isLoading: true })
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    expect(screen.queryByText('Loading backlog…')).not.toBeInTheDocument()
    expect(screen.queryByText(/Nothing here yet/)).not.toBeInTheDocument()
  })

  it('shows no empty state before data arrives', () => {
    mockBacklog({})
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    expect(screen.queryByText(/Nothing here yet/)).not.toBeInTheDocument()
  })

  it('offers no load more for an empty page', () => {
    mockBacklog({ data: page([], true) })
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  })

  it('follows new data from SWR', () => {
    mockBacklog({ data: page([matrix]) })
    const { rerender } = render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    mockBacklog({ data: page([avatar]) })
    rerender(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    expect(screen.getByText('Avatar 5')).toBeInTheDocument()
    expect(screen.queryByText('The Matrix')).not.toBeInTheDocument()
  })
})

describe('MoviesBacklog pagination on revalidation', () => {
  async function loadSecondPage() {
    mockFetchPage.mockResolvedValue({ items: [dune], hasMore: false })
    mockBacklog({ data: page([matrix], true) })
    const view = render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Load more' })))
    expect(screen.getByText('Dune')).toBeInTheDocument()
    return view
  }

  it('keeps loaded pages when the first page is unchanged', async () => {
    const { rerender } = await loadSecondPage()
    mockBacklog({ data: page([matrix], true) })
    rerender(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
  })

  it('drops loaded pages when the first page changed', async () => {
    const { rerender } = await loadSecondPage()
    mockBacklog({ data: page([avatar], true) })
    rerender(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)
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
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)

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
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)

    await act(async () =>
      fireEvent.change(screen.getByLabelText('Status of Dark'), { target: { value: 'watched' } })
    )
    expect(mockSetStatus).toHaveBeenCalledWith('s-1', 'watched', false)
  })

  it('asks only for watched', async () => {
    mockSetStatus.mockResolvedValue(undefined)
    mockBacklog({ data: page([series(2)]) })
    render(<MoviesBacklog filter={DEFAULT_BACKLOG_FILTER} />)

    await act(async () =>
      fireEvent.change(screen.getByLabelText('Status of Dark'), { target: { value: 'dropped' } })
    )
    expect(mockSetStatus).toHaveBeenCalledWith('s-1', 'dropped', false)
  })
})

import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockFetchPage = vi.fn()

vi.mock('@/hooks/useMovies', () => ({
  useMoviesBacklog: vi.fn(),
  useFetchMoviesBacklogPage: vi.fn(() => mockFetchPage)
}))

import MoviesBacklog from '@/components/movies/MoviesBacklog'
import MoviesBacklogFilters from '@/components/movies/MoviesBacklogFilters'
import { DEFAULT_BACKLOG_FILTER, type BacklogFilter } from '@/lib/movies/format'
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

const props = (filter: BacklogFilter = DEFAULT_BACKLOG_FILTER) => ({
  filter,
  onFilterChange: vi.fn(),
  addAction: <button>Add a title</button>
})

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

describe('MoviesBacklog toolbar', () => {
  it.each([
    [[matrix], '1 result'],
    [[matrix, avatar], '2 results']
  ])('counts the titles shown %#', (entries, label) => {
    mockBacklog({ data: page(entries) })
    render(<MoviesBacklog {...props()} />)
    expect(screen.getByRole('status')).toHaveTextContent(new RegExp(`^${label}$`))
  })

  it('filters the loaded titles by name as you type', () => {
    mockBacklog({ data: page([matrix, avatar, dune]) })
    render(<MoviesBacklog {...props()} />)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search titles' }), {
      target: { value: '  MAT ' }
    })
    expect(screen.getByText('The Matrix')).toBeInTheDocument()
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent(/^1 result$/)
  })

  it('says so when no title matches the search', () => {
    mockBacklog({ data: page([matrix]) })
    render(<MoviesBacklog {...props()} />)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search titles' }), {
      target: { value: 'zzz' }
    })
    expect(screen.getByText('No titles match.')).toBeInTheDocument()
  })

  it('passes filter changes up', () => {
    mockBacklog({ data: page([matrix]) })
    const p = props()
    render(<MoviesBacklog {...p} />)
    fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'series' } })
    expect(p.onFilterChange).toHaveBeenCalledWith({
      ...DEFAULT_BACKLOG_FILTER,
      mediaType: 'series'
    })
  })

  it.each([
    ['a status', { status: 'watched' }],
    ['a type', { mediaType: 'movie' }],
    ['the services chip', { onMyServices: true }]
  ])('keeps the toolbar when %s leaves no titles', (_name, patch) => {
    mockBacklog({ data: page([]) })
    render(<MoviesBacklog {...props({ ...DEFAULT_BACKLOG_FILTER, ...patch })} />)
    expect(screen.getByRole('searchbox', { name: 'Search titles' })).toBeInTheDocument()
    expect(screen.getByText('No titles match.')).toBeInTheDocument()
    expect(screen.queryByText(/Nothing here yet/)).not.toBeInTheDocument()
  })

  it('does not count the sort as a filter', () => {
    mockBacklog({ data: page([]) })
    render(<MoviesBacklog {...props({ ...DEFAULT_BACKLOG_FILTER, sort: 'title' })} />)
    expect(screen.getByText(/Nothing here yet/)).toBeInTheDocument()
  })
})

describe('MoviesBacklog rows', () => {
  it('shows type and release, or type alone when undated', () => {
    mockBacklog({ data: page([matrix, avatar]) })
    render(<MoviesBacklog {...props()} />)
    const metas = screen.getAllByText(/^Movie/, { selector: 'p' })
    expect(metas.map((p) => p.textContent)).toEqual(['Movie · 1999', 'Movie'])
  })

  it('shows the status as a badge, not a control', () => {
    mockBacklog({ data: page([matrix]) })
    render(<MoviesBacklog {...props()} />)
    expect(screen.getByRole('link', { name: /The Matrix/ })).toHaveTextContent('Want')
    expect(screen.queryByLabelText('Status of The Matrix')).not.toBeInTheDocument()
  })

  it('badges a series with a new season', () => {
    const caughtUp = create(BacklogEntrySchema, { ...matrix, hasNewSeason: false })
    const behind = create(BacklogEntrySchema, { ...avatar, hasNewSeason: true })
    mockBacklog({ data: page([caughtUp, behind]) })
    render(<MoviesBacklog {...props()} />)
    const [first, second] = screen.getAllByRole('link')
    expect(first).not.toHaveTextContent('New season')
    expect(second).toHaveTextContent('New season')
    expect(screen.queryByRole('img', { name: 'No rating' })).not.toBeInTheDocument()
  })

  it('shows a rating read-only, and nothing when unrated', () => {
    const rated = create(BacklogEntrySchema, { ...matrix, rating: 4 })
    mockBacklog({ data: page([rated, avatar]) })
    render(<MoviesBacklog {...props()} />)
    expect(screen.getByRole('img', { name: '4 out of 5 stars' })).toBeInTheDocument()
    expect(screen.getAllByRole('img', { name: /stars|No rating/ })).toHaveLength(1)
    expect(screen.queryByRole('button', { name: /^Rate/ })).not.toBeInTheDocument()
  })
})

describe('MoviesBacklog states', () => {
  it('offers the add action when the backlog is empty', () => {
    mockBacklog({ data: page([]) })
    render(<MoviesBacklog {...props()} />)
    expect(screen.getByText('Nothing here yet. Add your first title.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add a title' })).toBeInTheDocument()
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
  })

  it('shows no loading or empty state alongside data', () => {
    mockBacklog({ data: page([matrix]), isLoading: true })
    render(<MoviesBacklog {...props()} />)
    expect(screen.queryByText('Loading backlog…')).not.toBeInTheDocument()
    expect(screen.queryByText(/Nothing here yet/)).not.toBeInTheDocument()
  })

  it('shows no empty state before data arrives', () => {
    mockBacklog({})
    render(<MoviesBacklog {...props()} />)
    expect(screen.queryByText(/Nothing here yet/)).not.toBeInTheDocument()
  })

  it('offers no load more for an empty page', () => {
    mockBacklog({ data: page([], true) })
    render(<MoviesBacklog {...props()} />)
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  })

  it('follows new data from SWR', () => {
    mockBacklog({ data: page([matrix]) })
    const { rerender } = render(<MoviesBacklog {...props()} />)
    mockBacklog({ data: page([avatar]) })
    rerender(<MoviesBacklog {...props()} />)
    expect(screen.getByText('Avatar 5')).toBeInTheDocument()
    expect(screen.queryByText('The Matrix')).not.toBeInTheDocument()
  })
})

describe('MoviesBacklog pagination on revalidation', () => {
  async function loadSecondPage() {
    mockFetchPage.mockResolvedValue({ items: [dune], hasMore: false })
    mockBacklog({ data: page([matrix], true) })
    const view = render(<MoviesBacklog {...props()} />)
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Load more' })))
    expect(screen.getByText('Dune')).toBeInTheDocument()
    return view
  }

  it('keeps loaded pages when the first page is unchanged', async () => {
    const { rerender } = await loadSecondPage()
    mockBacklog({ data: page([matrix], true) })
    rerender(<MoviesBacklog {...props()} />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
  })

  it('drops loaded pages when the first page changed', async () => {
    const { rerender } = await loadSecondPage()
    mockBacklog({ data: page([avatar], true) })
    rerender(<MoviesBacklog {...props()} />)
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
  })
})

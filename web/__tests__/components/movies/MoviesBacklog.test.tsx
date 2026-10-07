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
})

describe('MoviesBacklog rows', () => {
  it('shows type and release, or type alone when undated', () => {
    mockBacklog({ data: page([matrix, avatar]) })
    render(<MoviesBacklog />)
    const metas = screen.getAllByText(/^Movie/, { selector: 'p' })
    expect(metas.map((p) => p.textContent)).toEqual(['Movie · 1999', 'Movie'])
    expect(screen.getByLabelText('Status of The Matrix')).toHaveAttribute('id', 'status-e-1')
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

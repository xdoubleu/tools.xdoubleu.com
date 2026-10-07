import React from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockSetStatus = jest.fn()
const mockRemove = jest.fn()
const mockPush = jest.fn()

jest.mock('@/hooks/useMovies', () => ({
  useMovieTitle: jest.fn(),
  useMoviesActions: jest.fn(() => ({
    add: jest.fn(),
    setStatus: mockSetStatus,
    remove: mockRemove
  }))
}))

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush })
}))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

import MovieTitleClient from '@/components/movies/MovieTitleClient'
import { useMovieTitle } from '@/hooks/useMovies'
import {
  BacklogEntrySchema,
  GetTitleResponseSchema,
  SeasonSchema
} from '@/lib/gen/movies/v1/movies_pb'

function mockTitle(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useMovieTitle).mockReturnValue({ isLoading: false, ...value })
}

const spirited = create(GetTitleResponseSchema, {
  overview: 'A girl in a spirit world.',
  entry: create(BacklogEntrySchema, {
    id: 'e-1',
    mediaType: 'movie',
    title: 'Spirited Away',
    originalTitle: '千と千尋の神隠し',
    releaseDate: '2001-07-20',
    genres: ['Animation', 'Fantasy'],
    runtime: 125,
    status: 'watched',
    watchedAt: [new Date(2026, 9, 7, 12).toISOString(), '']
  })
})

beforeEach(() => jest.clearAllMocks())

describe('MovieTitleClient', () => {
  it('shows the title details and watch dates', () => {
    mockTitle({ data: spirited })
    render(<MovieTitleClient id="e-1" />)

    expect(screen.getByRole('heading', { name: 'Spirited Away' })).toBeInTheDocument()
    expect(screen.getByText('千と千尋の神隠し')).toBeInTheDocument()
    expect(screen.getByText('Movie · 2001 · 125 min')).toBeInTheDocument()
    expect(screen.getByText('Fantasy')).toBeInTheDocument()
    expect(screen.getByText('A girl in a spirit world.')).toBeInTheDocument()
    expect(screen.getByLabelText('Watch 1 date')).toHaveValue('2026-10-07')
    expect(screen.getByLabelText('Watch 2 date')).toHaveValue('')
    expect(screen.getByText('Date unknown')).toBeInTheDocument()
    expect(screen.getByLabelText('Status')).toHaveValue('watched')
  })

  it('shows the season count for series', () => {
    mockTitle({
      data: create(GetTitleResponseSchema, {
        entry: create(BacklogEntrySchema, {
          id: 'e-2',
          mediaType: 'series',
          title: 'Breaking Bad',
          originalTitle: 'Breaking Bad',
          seasonCount: 1,
          status: 'want'
        })
      })
    })
    render(<MovieTitleClient id="e-2" />)
    expect(screen.getByText('Series · 1 season')).toBeInTheDocument()
    expect(screen.queryByText('Watched', { selector: 'p' })).not.toBeInTheDocument()
  })

  it('changes status', async () => {
    mockSetStatus.mockResolvedValue(undefined)
    mockTitle({ data: spirited })
    render(<MovieTitleClient id="e-1" />)

    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'dropped' } })
    await waitFor(() => expect(mockSetStatus).toHaveBeenCalledWith('e-1', 'dropped', false))
  })

  it('removes after confirming, then returns to the backlog', async () => {
    mockRemove.mockResolvedValue(undefined)
    mockTitle({ data: spirited })
    render(<MovieTitleClient id="e-1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove from backlog' }))
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Remove' })))
    expect(mockRemove).toHaveBeenCalledWith('e-1')
    expect(mockPush).toHaveBeenCalledWith('/movies')
  })

  it('reports a failed status change', async () => {
    mockSetStatus.mockRejectedValue(new Error('nope'))
    mockTitle({ data: spirited })
    render(<MovieTitleClient id="e-1" />)

    await act(async () =>
      fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'dropped' } })
    )
    expect(screen.getByText(/Couldn.t update status/)).toBeInTheDocument()
  })

  it('reports a failed remove and stays on the page', async () => {
    mockRemove.mockRejectedValue(new Error('nope'))
    mockTitle({ data: spirited })
    render(<MovieTitleClient id="e-1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove from backlog' }))
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Remove' })))
    expect(screen.getByText(/Couldn.t remove/)).toBeInTheDocument()
    expect(mockPush).not.toHaveBeenCalled()
  })

  it('shows loading and error states', () => {
    mockTitle({ isLoading: true })
    const { rerender } = render(<MovieTitleClient id="e-1" />)
    expect(screen.getByText('Loading title…')).toBeInTheDocument()

    mockTitle({ error: new Error('nope') })
    rerender(<MovieTitleClient id="e-1" />)
    expect(screen.getByText('Failed to load title.')).toBeInTheDocument()
    // Heading plus the breadcrumb's current page.
    expect(screen.getAllByText('Movies & Series')).toHaveLength(2)

    mockTitle({})
    rerender(<MovieTitleClient id="e-1" />)
    expect(screen.getByText('Failed to load title.')).toBeInTheDocument()
  })

  it('keeps showing loaded data when revalidation fails', () => {
    mockTitle({ data: spirited, error: new Error('offline') })
    render(<MovieTitleClient id="e-1" />)
    expect(screen.getByRole('heading', { name: 'Spirited Away' })).toBeInTheDocument()
  })

  it('stops showing a title removed elsewhere', () => {
    mockTitle({ data: spirited, error: new ConnectError('gone', Code.NotFound) })
    render(<MovieTitleClient id="e-1" />)
    expect(screen.getByText('Failed to load title.')).toBeInTheDocument()
  })

  it('links back and names the title in the breadcrumb and dialog', () => {
    mockTitle({ data: spirited })
    render(<MovieTitleClient id="e-1" />)
    expect(screen.getByRole('link', { name: 'Movies & Series' })).toHaveAttribute('href', '/movies')
    expect(screen.getAllByText('Spirited Away')).toHaveLength(2)

    fireEvent.click(screen.getByRole('button', { name: 'Remove from backlog' }))
    expect(screen.getByText('Remove Spirited Away?')).toBeInTheDocument()
    expect(screen.queryByText(/Couldn.t remove/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Couldn.t update status/)).not.toBeInTheDocument()
  })

  it('leaves out what a bare series lacks', () => {
    mockTitle({
      data: create(GetTitleResponseSchema, {
        entry: create(BacklogEntrySchema, {
          id: 'e-3',
          mediaType: 'series',
          title: 'Dark',
          originalTitle: 'Dark',
          seasonCount: 3,
          status: 'want'
        })
      })
    })
    const { container } = render(<MovieTitleClient id="e-3" />)
    expect(screen.getByText('Series · 3 seasons')).toBeInTheDocument()
    // Heading and breadcrumb only: an identical original title isn't repeated.
    expect(screen.getAllByText('Dark')).toHaveLength(2)
    expect(container.querySelector('div.flex-wrap.gap-2')).toBeNull()
    expect(container.querySelector('p.text-sm:empty')).toBeNull()
  })

  it('locks the status while saving and clears an old error on success', async () => {
    mockTitle({ data: spirited })
    render(<MovieTitleClient id="e-1" />)
    const select = () => screen.getByLabelText('Status')
    expect(select()).toBeEnabled()

    mockSetStatus.mockRejectedValueOnce(new Error('nope'))
    await act(async () => fireEvent.change(select(), { target: { value: 'want' } }))
    expect(screen.getByText(/Couldn.t update status/)).toBeInTheDocument()
    expect(select()).toBeEnabled()

    let resolve!: () => void
    mockSetStatus.mockReturnValueOnce(new Promise<void>((res) => (resolve = res)))
    fireEvent.change(select(), { target: { value: 'want' } })
    expect(select()).toBeDisabled()
    expect(screen.queryByText(/Couldn.t update status/)).not.toBeInTheDocument()
    await act(async () => resolve())
    expect(select()).toBeEnabled()
  })

  it('shows removing progress and clears an old error on retry', async () => {
    mockTitle({ data: spirited })
    render(<MovieTitleClient id="e-1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Remove from backlog' }))

    mockRemove.mockRejectedValueOnce(new Error('nope'))
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Remove' })))
    expect(screen.getByText(/Couldn.t remove/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument()

    let resolve!: () => void
    mockRemove.mockReturnValueOnce(new Promise<void>((res) => (resolve = res)))
    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    expect(screen.getByRole('button', { name: 'Removing…' })).toBeInTheDocument()
    expect(screen.queryByText(/Couldn.t remove/)).not.toBeInTheDocument()
    await act(async () => resolve())
    expect(mockPush).toHaveBeenCalledWith('/movies')
  })
})

describe('MovieTitleClient series', () => {
  it('shows the season checklist instead of movie watches', () => {
    mockTitle({
      data: create(GetTitleResponseSchema, {
        entry: create(BacklogEntrySchema, {
          id: 's-1',
          mediaType: 'series',
          title: 'Dark',
          status: 'watching'
        }),
        seasons: [create(SeasonSchema, { number: 1, name: 'Season 1', aired: true })]
      })
    })
    render(<MovieTitleClient id="s-1" />)
    expect(screen.getByRole('region', { name: 'Seasons' })).toBeInTheDocument()
    expect(screen.getByLabelText('Season 1')).not.toBeChecked()
    expect(screen.queryByRole('region', { name: 'Watched' })).not.toBeInTheDocument()
  })
})

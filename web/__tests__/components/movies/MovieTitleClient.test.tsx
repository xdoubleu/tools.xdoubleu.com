import React from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

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
import { BacklogEntrySchema, GetTitleResponseSchema } from '@/lib/gen/movies/v1/movies_pb'

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
    expect(screen.getByText('2026-10-07')).toBeInTheDocument()
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
    await waitFor(() => expect(mockSetStatus).toHaveBeenCalledWith('e-1', 'dropped'))
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
  })
})

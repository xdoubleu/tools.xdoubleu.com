import React from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockRemove = vi.fn()

vi.mock('@/hooks/useTasteShows', () => ({
  useShowFavourites: vi.fn(),
  useShowActions: vi.fn(() => ({ remove: mockRemove }))
}))
vi.mock('@/components/ui/artwork', () => ({
  Artwork: () => <div data-testid="artwork" />
}))

import ShowFavourites from '@/components/taste/ShowFavourites'
import { useShowFavourites } from '@/hooks/useTasteShows'
import { FavouriteShowSchema, ListFavouriteShowsResponseSchema } from '@/lib/gen/taste/v1/taste_pb'

const list = create(ListFavouriteShowsResponseSchema, {
  favourites: [
    create(FavouriteShowSchema, { id: 'f-1', title: 'Hardcore History', author: 'Dan Carlin' }),
    create(FavouriteShowSchema, { id: 'f-2', title: 'History of Rome' })
  ]
})

function mockFavourites(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useShowFavourites).mockReturnValue({ isLoading: false, ...value })
}

beforeEach(() => {
  vi.clearAllMocks()
  mockRemove.mockResolvedValue(undefined)
})

describe('ShowFavourites', () => {
  it('lists favourites, with an author line only when there is an author', () => {
    mockFavourites({ data: list })
    const { container } = render(<ShowFavourites />)
    expect(screen.getByText('Hardcore History')).toBeInTheDocument()
    expect(screen.getByText('Dan Carlin').tagName).toBe('P')
    expect(screen.getByText('History of Rome')).toBeInTheDocument()
    expect([...container.querySelectorAll('p')].filter((p) => !p.textContent)).toHaveLength(0)
    expect(screen.queryByText(/Couldn.t remove/)).not.toBeInTheDocument()
  })

  it('keeps listing while revalidating', () => {
    mockFavourites({ data: list, isLoading: true })
    render(<ShowFavourites />)
    expect(screen.getByText('Hardcore History')).toBeInTheDocument()
    expect(screen.queryByText(/Loading favourites/)).not.toBeInTheDocument()
  })

  const confirmRemove = () => {
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0])
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Remove' }))
  }

  it('asks before removing a favourite, then removes it', async () => {
    mockFavourites({ data: list })
    render(<ShowFavourites />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0])
    expect(screen.getByRole('dialog', { name: 'Remove favourite?' })).toBeInTheDocument()
    expect(
      screen.getByText('Hardcore History leaves your favourites. You can add it again from search.')
    ).toBeInTheDocument()
    expect(mockRemove).not.toHaveBeenCalled()

    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Remove' }))
    expect(screen.getByRole('button', { name: 'Removing…' })).toBeDisabled()
    await waitFor(() => expect(mockRemove).toHaveBeenCalledWith('f-1'))
    expect(screen.queryByText(/Couldn.t remove/)).not.toBeInTheDocument()
  })

  it('keeps the favourite when the confirmation is cancelled', () => {
    mockFavourites({ data: list })
    render(<ShowFavourites />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0])
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(mockRemove).not.toHaveBeenCalled()
  })

  it('clears an earlier failure when retrying', async () => {
    mockRemove.mockRejectedValueOnce(new Error('boom'))
    mockFavourites({ data: list })
    render(<ShowFavourites />)
    confirmRemove()
    expect(await screen.findByText(/Couldn.t remove/)).toBeInTheDocument()
    confirmRemove()
    await waitFor(() => expect(screen.queryByText(/Couldn.t remove/)).not.toBeInTheDocument())
  })

  it('reports a failed removal and closes the confirmation', async () => {
    mockRemove.mockRejectedValue(new Error('boom'))
    mockFavourites({ data: list })
    render(<ShowFavourites />)
    confirmRemove()
    expect(await screen.findByText(/Couldn.t remove/)).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('filters by title or author and says so when nothing matches', () => {
    mockFavourites({ data: list })
    render(<ShowFavourites />)
    const search = screen.getByRole('searchbox', { name: 'Search shows' })

    fireEvent.change(search, { target: { value: '  Carlin ' } })
    expect(screen.getByText('Hardcore History')).toBeInTheDocument()
    expect(screen.queryByText('History of Rome')).not.toBeInTheDocument()

    fireEvent.change(search, { target: { value: 'rome' } })
    expect(screen.getByText('History of Rome')).toBeInTheDocument()
    expect(screen.queryByText('Hardcore History')).not.toBeInTheDocument()

    fireEvent.change(search, { target: { value: 'zzz' } })
    expect(screen.getByText('No shows match.')).toBeInTheDocument()
  })

  it('shows a loading state', () => {
    mockFavourites({ isLoading: true })
    render(<ShowFavourites />)
    expect(screen.getByText(/Loading favourites/)).toBeInTheDocument()
  })

  it('shows an empty state', () => {
    mockFavourites({ data: create(ListFavouriteShowsResponseSchema, { favourites: [] }) })
    render(<ShowFavourites />)
    expect(screen.getByText(/No favourite shows yet/)).toBeInTheDocument()
  })

  it('shows a load error', () => {
    mockFavourites({ error: new Error('boom') })
    render(<ShowFavourites />)
    expect(screen.getByText(/Failed to load favourites/)).toBeInTheDocument()
  })
})

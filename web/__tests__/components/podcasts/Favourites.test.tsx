import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockRemove = jest.fn()

jest.mock('@/hooks/usePodcasts', () => ({
  usePodcastFavourites: jest.fn(),
  usePodcastActions: jest.fn(() => ({ remove: mockRemove }))
}))
jest.mock('@/components/podcasts/ShowArtwork', () => ({
  __esModule: true,
  default: () => <div data-testid="artwork" />
}))

import Favourites from '@/components/podcasts/Favourites'
import { usePodcastFavourites } from '@/hooks/usePodcasts'
import { FavouriteSchema, ListFavouritesResponseSchema } from '@/lib/gen/podcasts/v1/podcasts_pb'

const list = create(ListFavouritesResponseSchema, {
  favourites: [
    create(FavouriteSchema, { id: 'f-1', title: 'Hardcore History', author: 'Dan Carlin' }),
    create(FavouriteSchema, {
      id: 'f-2',
      title: 'History of Rome',
      fetchError: 'feed answered HTTP 404'
    })
  ]
})

function mockFavourites(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(usePodcastFavourites).mockReturnValue({ isLoading: false, ...value })
}

beforeEach(() => {
  jest.clearAllMocks()
  mockRemove.mockResolvedValue(undefined)
})

describe('Favourites', () => {
  it('lists favourites, with an author line only when there is an author', () => {
    mockFavourites({ data: list })
    const { container } = render(<Favourites />)
    expect(screen.getByText('Hardcore History')).toBeInTheDocument()
    expect(screen.getByText('Dan Carlin').tagName).toBe('P')
    expect(screen.getByText('History of Rome')).toBeInTheDocument()
    expect([...container.querySelectorAll('p')].filter((p) => !p.textContent)).toHaveLength(0)
    expect(screen.queryByText(/Couldn.t remove/)).not.toBeInTheDocument()
  })

  it("says when a show's episodes could not be fetched", () => {
    mockFavourites({ data: list })
    render(<Favourites />)
    expect(screen.getAllByText(/Episodes unavailable/)).toHaveLength(1)
    expect(screen.getByText('Episodes unavailable: feed answered HTTP 404.')).toBeInTheDocument()
  })

  it('keeps listing while revalidating', () => {
    mockFavourites({ data: list, isLoading: true })
    render(<Favourites />)
    expect(screen.getByText('Hardcore History')).toBeInTheDocument()
    expect(screen.queryByText(/Loading favourites/)).not.toBeInTheDocument()
  })

  it('removes a favourite', async () => {
    mockFavourites({ data: list })
    render(<Favourites />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0])
    expect(screen.getByRole('button', { name: 'Removing…' })).toBeDisabled()
    await waitFor(() => expect(mockRemove).toHaveBeenCalledWith('f-1'))
    expect(screen.queryByText(/Couldn.t remove/)).not.toBeInTheDocument()
  })

  it('clears an earlier failure when retrying', async () => {
    mockRemove.mockRejectedValueOnce(new Error('boom'))
    mockFavourites({ data: list })
    render(<Favourites />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0])
    expect(await screen.findByText(/Couldn.t remove/)).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0])
    await waitFor(() => expect(screen.queryByText(/Couldn.t remove/)).not.toBeInTheDocument())
  })

  it('reports a failed removal', async () => {
    mockRemove.mockRejectedValue(new Error('boom'))
    mockFavourites({ data: list })
    render(<Favourites />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0])
    expect(await screen.findByText(/Couldn.t remove/)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Remove' })).toHaveLength(2)
  })

  it('shows a loading state', () => {
    mockFavourites({ isLoading: true })
    render(<Favourites />)
    expect(screen.getByText(/Loading favourites/)).toBeInTheDocument()
  })

  it('shows an empty state', () => {
    mockFavourites({ data: create(ListFavouritesResponseSchema, { favourites: [] }) })
    render(<Favourites />)
    expect(screen.getByText(/No favourite podcasts yet/)).toBeInTheDocument()
  })

  it('shows a load error', () => {
    mockFavourites({ error: new Error('boom') })
    render(<Favourites />)
    expect(screen.getByText(/Failed to load favourites/)).toBeInTheDocument()
  })
})

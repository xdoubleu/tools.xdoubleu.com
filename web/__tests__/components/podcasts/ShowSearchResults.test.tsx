import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'

const mockAdd = jest.fn()

jest.mock('@/hooks/usePodcasts', () => ({
  usePodcastSearch: jest.fn(),
  usePodcastActions: jest.fn(() => ({ add: mockAdd }))
}))
jest.mock('@/components/podcasts/ShowArtwork', () => ({
  __esModule: true,
  default: () => <div data-testid="artwork" />
}))

import ShowSearchResults from '@/components/podcasts/ShowSearchResults'
import { usePodcastSearch } from '@/hooks/usePodcasts'
import { SearchResultSchema, SearchShowsResponseSchema } from '@/lib/gen/podcasts/v1/podcasts_pb'

const results = create(SearchShowsResponseSchema, {
  results: [
    create(SearchResultSchema, { itunesId: 1n, title: 'Hardcore History', author: 'Dan Carlin' }),
    create(SearchResultSchema, { itunesId: 2n, title: 'History of Rome', favourite: true })
  ]
})

function mockSearch(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(usePodcastSearch).mockReturnValue({ isLoading: false, ...value })
}

beforeEach(() => {
  jest.clearAllMocks()
  mockAdd.mockResolvedValue(undefined)
})

describe('ShowSearchResults', () => {
  it('lists results, marking favourites', () => {
    mockSearch({ data: results })
    const { container } = render(<ShowSearchResults query="history" />)
    expect(screen.getByText('Hardcore History')).toBeInTheDocument()
    expect(screen.getByText('Dan Carlin').tagName).toBe('P')
    expect([...container.querySelectorAll('p')].filter((p) => !p.textContent)).toHaveLength(0)
    expect(screen.queryByText(/Couldn.t add/)).not.toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Add' })).toHaveLength(1)
    expect(screen.getByText('Favourite')).toBeInTheDocument()
  })

  it('adds a show', async () => {
    mockSearch({ data: results })
    render(<ShowSearchResults query="history" />)
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(screen.getByRole('button', { name: 'Adding…' })).toBeDisabled()
    await waitFor(() => expect(mockAdd).toHaveBeenCalledWith(1n))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Add' })).toBeEnabled())
  })

  it('keeps listing while revalidating', () => {
    mockSearch({ data: results, isLoading: true })
    render(<ShowSearchResults query="history" />)
    expect(screen.getByText('Hardcore History')).toBeInTheDocument()
    expect(screen.queryByText(/Loading results/)).not.toBeInTheDocument()
  })

  it('clears an earlier failure when retrying', async () => {
    mockAdd.mockRejectedValueOnce(new Error('boom'))
    mockSearch({ data: results })
    render(<ShowSearchResults query="history" />)
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(await screen.findByText(/Couldn.t add/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(screen.queryByText(/Couldn.t add/)).not.toBeInTheDocument())
  })

  it('reports a failed add', async () => {
    mockAdd.mockRejectedValue(new Error('boom'))
    mockSearch({ data: results })
    render(<ShowSearchResults query="history" />)
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(await screen.findByText(/Couldn.t add/)).toBeInTheDocument()
  })

  it('shows a loading state', () => {
    mockSearch({ isLoading: true })
    render(<ShowSearchResults query="history" />)
    expect(screen.getByText(/Loading results/)).toBeInTheDocument()
  })

  it('shows an empty state', () => {
    mockSearch({ data: create(SearchShowsResponseSchema, { results: [] }) })
    render(<ShowSearchResults query=" nothing " />)
    expect(screen.getByText(/No podcasts match/)).toHaveTextContent('“nothing”')
  })

  it('explains an unavailable iTunes', () => {
    mockSearch({ error: new ConnectError('down', Code.Unavailable) })
    render(<ShowSearchResults query="history" />)
    expect(screen.getByText(/iTunes is unavailable/)).toBeInTheDocument()
  })

  it('shows a generic search error', () => {
    mockSearch({ error: new Error('boom') })
    render(<ShowSearchResults query="history" />)
    expect(screen.getByText('Search failed. Try again.')).toBeInTheDocument()
  })
})

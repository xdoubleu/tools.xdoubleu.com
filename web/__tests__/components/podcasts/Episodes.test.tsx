import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockFetchPage = jest.fn()

jest.mock('@/hooks/usePodcasts', () => ({
  usePodcastEpisodes: jest.fn(),
  useFetchPodcastEpisodesPage: jest.fn(() => mockFetchPage)
}))
jest.mock('@/components/podcasts/ShowArtwork', () => ({
  __esModule: true,
  default: ({ title }: { title: string }) => <div data-testid="artwork" data-title={title} />
}))

import Episodes from '@/components/podcasts/Episodes'
import { usePodcastEpisodes } from '@/hooks/usePodcasts'
import {
  type Episode,
  EpisodeSchema,
  ListEpisodesResponseSchema
} from '@/lib/gen/podcasts/v1/podcasts_pb'

const full = create(EpisodeSchema, {
  id: 'e-1',
  showId: 's-1',
  showTitle: 'Hardcore History',
  title: 'Supernova in the East I',
  summary: 'The Pacific war.',
  link: 'https://shows.example/hh/70',
  appleUrl: 'https://podcasts.apple.com/podcast/id1',
  durationSeconds: 3723,
  publishedAt: '2026-10-05T08:00:00Z'
})
const bare = create(EpisodeSchema, { id: 'e-2', showTitle: 'Rome', title: 'Bonus' })

function mockEpisodes(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(usePodcastEpisodes).mockReturnValue({ isLoading: false, ...value })
}

function page(episodes: Episode[], hasMore = false) {
  return create(ListEpisodesResponseSchema, { episodes, hasMore })
}

beforeEach(() => jest.clearAllMocks())

describe('Episodes', () => {
  it('shows an episode with its show, date, duration, summary and links', () => {
    mockEpisodes({ data: page([full]) })
    render(<Episodes />)
    expect(screen.getByText('Supernova in the East I')).toBeInTheDocument()
    expect(screen.getByText('Hardcore History · 05/10/2026 · 1h 2m')).toBeInTheDocument()
    expect(screen.getByText('The Pacific war.').tagName).toBe('P')
    expect(screen.getByText('The Pacific war.')).toHaveClass('line-clamp-3')
    expect(screen.getByRole('link', { name: 'Episode page' })).toHaveClass('min-h-11')
    expect(screen.getByRole('link', { name: 'Episode page' })).toHaveAttribute(
      'href',
      'https://shows.example/hh/70'
    )
    expect(screen.getByRole('link', { name: 'Open in Apple Podcasts' })).toHaveAttribute(
      'href',
      'https://podcasts.apple.com/podcast/id1'
    )
    expect(screen.getByTestId('artwork')).toHaveAttribute('data-title', 'Hardcore History')
  })

  it('leaves out what the feed did not give', () => {
    mockEpisodes({ data: page([bare]) })
    const { container } = render(<Episodes />)
    expect(screen.getByText('Rome')).toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.queryByText('·')).not.toBeInTheDocument()
    expect(container.querySelector('p.line-clamp-3')).not.toBeInTheDocument()
  })

  it('loads more episodes', async () => {
    mockFetchPage.mockResolvedValue({ items: [bare], hasMore: false })
    mockEpisodes({ data: page([full], true) })
    render(<Episodes />)
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    await waitFor(() => expect(mockFetchPage).toHaveBeenCalledWith(1))
    expect(await screen.findByText('Bonus')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  })

  it('has no load-more button on the last page', () => {
    mockEpisodes({ data: page([full]) })
    render(<Episodes />)
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  })

  it('shows a loading state', () => {
    mockEpisodes({ isLoading: true })
    render(<Episodes />)
    expect(screen.getByText(/Loading episodes/)).toBeInTheDocument()
  })

  it('keeps listing while revalidating', () => {
    mockEpisodes({ data: page([full]), isLoading: true })
    render(<Episodes />)
    expect(screen.getByText('Supernova in the East I')).toBeInTheDocument()
    expect(screen.queryByText(/Loading episodes/)).not.toBeInTheDocument()
  })

  it('shows the empty state while there is neither data nor an error', () => {
    mockEpisodes({})
    render(<Episodes />)
    expect(screen.getByText(/No episodes yet/)).toBeInTheDocument()
  })

  it('follows fresh data', () => {
    mockEpisodes({ data: page([full]) })
    const { rerender } = render(<Episodes />)
    mockEpisodes({ data: page([bare]) })
    rerender(<Episodes />)
    expect(screen.getByText('Bonus')).toBeInTheDocument()
    expect(screen.queryByText('Supernova in the East I')).not.toBeInTheDocument()
  })

  it('keeps loaded pages when a revalidation returns the same first episodes', async () => {
    mockFetchPage.mockResolvedValue({ items: [bare], hasMore: false })
    mockEpisodes({ data: page([full], true) })
    const { rerender } = render(<Episodes />)
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(await screen.findByText('Bonus')).toBeInTheDocument()

    mockEpisodes({ data: page([create(EpisodeSchema, { ...full }) as Episode], true) })
    rerender(<Episodes />)
    expect(screen.getByText('Bonus')).toBeInTheDocument()
    expect(screen.getByText('Supernova in the East I')).toBeInTheDocument()
  })

  it('drops loaded pages when a revalidation starts with different episodes', async () => {
    mockFetchPage.mockResolvedValue({ items: [bare], hasMore: false })
    mockEpisodes({ data: page([full], true) })
    const { rerender } = render(<Episodes />)
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(await screen.findByText('Bonus')).toBeInTheDocument()

    const fresh = create(EpisodeSchema, { id: 'e-9', showTitle: 'New', title: 'Brand new' })
    mockEpisodes({ data: page([fresh], true) })
    rerender(<Episodes />)
    expect(screen.getByText('Brand new')).toBeInTheDocument()
    expect(screen.queryByText('Bonus')).not.toBeInTheDocument()
    expect(screen.queryByText('Supernova in the East I')).not.toBeInTheDocument()
  })

  it('shows an empty state', () => {
    mockEpisodes({ data: page([]) })
    render(<Episodes />)
    expect(screen.getByText(/No episodes yet/)).toBeInTheDocument()
  })

  it('shows a load error', () => {
    mockEpisodes({ error: new Error('boom') })
    render(<Episodes />)
    expect(screen.getByText(/Failed to load episodes/)).toBeInTheDocument()
  })

  it('keeps listing when a revalidation fails', () => {
    mockEpisodes({ data: page([full]), error: new Error('boom') })
    render(<Episodes />)
    expect(screen.getByText('Supernova in the East I')).toBeInTheDocument()
    expect(screen.queryByText(/Failed to load/)).not.toBeInTheDocument()
  })
})

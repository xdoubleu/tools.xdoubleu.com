import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockUseFeeds = jest.fn()
const deleteFeed = jest.fn()
const refreshFeed = jest.fn()

jest.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => mockUseFeeds(),
  useDeleteFeed: () => deleteFeed,
  useRefreshFeed: () => refreshFeed,
  useCreateFeed: () => jest.fn()
}))
jest.mock('@/lib/gen/feeds/v1/feeds_pb', () => ({
  FeedKind: { RSS: 1, EMAIL: 2 }
}))

import FeedManager from '@/components/feeds/FeedManager'

const rssFeed = {
  id: 'feed-1',
  url: 'https://blog.example.com/feed.xml',
  title: 'Example Blog',
  lastFetchedAt: '',
  lastError: '',
  sourceType: 'rss'
}

const emailFeed = {
  id: 'feed-2',
  url: '',
  title: 'My Substack',
  lastFetchedAt: '',
  lastError: '',
  sourceType: 'email'
}

describe('FeedManager', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('shows a loading state', () => {
    mockUseFeeds.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    render(<FeedManager />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockUseFeeds.mockReturnValue({ data: undefined, error: new Error('nope'), isLoading: false })
    render(<FeedManager />)
    expect(screen.getByText('Failed to load feeds.')).toBeInTheDocument()
  })

  it('shows an empty state', () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [] }, error: undefined, isLoading: false })
    render(<FeedManager />)
    expect(screen.getByText('No feed subscriptions yet.')).toBeInTheDocument()
  })

  it('lists an RSS feed with a Refresh button and refreshes it', async () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [rssFeed] }, error: undefined, isLoading: false })
    refreshFeed.mockResolvedValue({ ingested: 2 })
    render(<FeedManager />)

    expect(screen.getByText('Example Blog')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))

    await waitFor(() => {
      expect(refreshFeed).toHaveBeenCalledWith('feed-1')
      expect(screen.getByText('Ingested 2 item(s).')).toBeInTheDocument()
    })
  })

  it('deletes a feed', async () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [rssFeed] }, error: undefined, isLoading: false })
    deleteFeed.mockResolvedValue(undefined)
    render(<FeedManager />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))

    await waitFor(() => {
      expect(deleteFeed).toHaveBeenCalledWith('feed-1')
    })
  })

  it('shows a failure status when an action rejects', async () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [rssFeed] }, error: undefined, isLoading: false })
    deleteFeed.mockRejectedValue(new Error('boom'))
    render(<FeedManager />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))

    await waitFor(() => {
      expect(screen.getByText('Action failed.')).toBeInTheDocument()
    })
  })

  it('renders an email feed without a Refresh button, waiting for its first email', () => {
    mockUseFeeds.mockReturnValue({
      data: { feeds: [emailFeed] },
      error: undefined,
      isLoading: false
    })
    render(<FeedManager />)

    expect(screen.getByText('My Substack')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Refresh' })).not.toBeInTheDocument()
    expect(screen.getByText(/Waiting for your first email/)).toBeInTheDocument()
  })
})

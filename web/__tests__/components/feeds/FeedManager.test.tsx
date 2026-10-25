import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

const mockUseFeeds = vi.fn()
const deleteFeed = vi.fn()
const refreshFeed = vi.fn()

vi.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => mockUseFeeds(),
  useDeleteFeed: () => deleteFeed,
  useRefreshFeed: () => refreshFeed,
  useCreateFeed: () => vi.fn()
}))
vi.mock('@/lib/gen/feeds/v1/feeds_pb', () => ({
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
    vi.clearAllMocks()
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

  it('deletes a feed only after confirming', async () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [rssFeed] }, error: undefined, isLoading: false })
    deleteFeed.mockResolvedValue(undefined)
    render(<FeedManager />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    const dialog = screen.getByRole('dialog', { name: 'Remove Example Blog?' })
    expect(dialog).toHaveTextContent(/items, bookmarks and filter rules/)
    expect(deleteFeed).not.toHaveBeenCalled()

    fireEvent.click(within(dialog).getByRole('button', { name: 'Remove' }))

    await waitFor(() => {
      expect(deleteFeed).toHaveBeenCalledWith('feed-1')
    })
  })

  it('keeps the feed when the confirm is cancelled', () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [rssFeed] }, error: undefined, isLoading: false })
    render(<FeedManager />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(deleteFeed).not.toHaveBeenCalled()
  })

  it('shows a failure status when an action rejects', async () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [rssFeed] }, error: undefined, isLoading: false })
    deleteFeed.mockRejectedValue(new Error('boom'))
    render(<FeedManager />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Remove' }))

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

  it.each([
    [{ ...rssFeed, title: '', lastError: 'timeout' }, rssFeed.url, 'Last poll failed: timeout'],
    [
      { ...emailFeed, title: '', lastError: 'bounced' },
      'Email newsletter',
      'Last email failed: bounced'
    ]
  ])('shows the last error for %#', (feed, name, errorText) => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [feed] }, error: undefined, isLoading: false })
    render(<FeedManager />)

    expect(screen.getAllByText(name)[0]).toBeInTheDocument()
    expect(screen.getByText(errorText)).toBeInTheDocument()
  })

  it.each([
    [{ ...rssFeed, lastFetchedAt: '2026-01-01T00:00:00Z' }, /^Last fetched/],
    [{ ...emailFeed, lastFetchedAt: '2026-01-01T00:00:00Z' }, /^Verified — last email received/]
  ])('shows when a feed was last fetched for %#', (feed, text) => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [feed] }, error: undefined, isLoading: false })
    render(<FeedManager />)

    expect(screen.getByText(text)).toBeInTheDocument()
  })
})

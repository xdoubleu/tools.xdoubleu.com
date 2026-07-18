import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import FeedManager from '@/components/reading/FeedManager'

const feedsData: { data?: unknown; error?: Error; isLoading: boolean } = {
  data: undefined,
  error: undefined,
  isLoading: false
}
const createFeed = jest.fn()
const updateFeed = jest.fn()
const deleteFeed = jest.fn()
const refreshFeed = jest.fn()

jest.mock('@/hooks/useBookFeeds', () => ({
  useFeeds: () => feedsData,
  useCreateFeed: () => createFeed,
  useUpdateFeed: () => updateFeed,
  useDeleteFeed: () => deleteFeed,
  useRefreshFeed: () => refreshFeed
}))

const feed = {
  id: 'feed-1',
  url: 'https://blog.example.com/feed.xml',
  title: 'Example Blog',
  koboSync: false,
  lastFetchedAt: '2026-07-17T08:00:00Z',
  lastError: '',
  createdAt: '2026-07-01T08:00:00Z'
}

describe('FeedManager', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    feedsData.data = { feeds: [feed] }
    feedsData.error = undefined
    feedsData.isLoading = false
  })

  it('lists feeds with title and url', () => {
    render(<FeedManager />)
    expect(screen.getByText('Example Blog')).toBeInTheDocument()
    expect(screen.getByText('https://blog.example.com/feed.xml')).toBeInTheDocument()
  })

  it('shows the last poll error when present', () => {
    feedsData.data = { feeds: [{ ...feed, lastError: 'fetch failed' }] }
    render(<FeedManager />)
    expect(screen.getByText(/Last poll failed: fetch failed/)).toBeInTheDocument()
  })

  it('subscribes to a new feed with kobo sync', async () => {
    createFeed.mockResolvedValue({ ingested: 3 })
    render(<FeedManager />)

    fireEvent.change(screen.getByLabelText('Feed URL'), {
      target: { value: 'https://news.example.com/rss' }
    })
    // Two "Kobo sync" checkboxes exist (add-form + row); the first belongs
    // to the add form.
    fireEvent.click(screen.getAllByRole('checkbox')[0])
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe' }))

    await waitFor(() =>
      expect(createFeed).toHaveBeenCalledWith('https://news.example.com/rss', true)
    )
    expect(await screen.findByText('Subscribed — imported 3 item(s).')).toBeInTheDocument()
  })

  it('toggles per-feed kobo sync', async () => {
    updateFeed.mockResolvedValue(undefined)
    render(<FeedManager />)

    fireEvent.click(screen.getAllByRole('checkbox')[1])
    await waitFor(() => expect(updateFeed).toHaveBeenCalledWith('feed-1', 'Example Blog', true))
  })

  it('refreshes and removes a feed', async () => {
    refreshFeed.mockResolvedValue({ ingested: 1 })
    deleteFeed.mockResolvedValue(undefined)
    render(<FeedManager />)

    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => expect(refreshFeed).toHaveBeenCalledWith('feed-1'))
    expect(await screen.findByText('Ingested 1 item(s).')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    await waitFor(() => expect(deleteFeed).toHaveBeenCalledWith('feed-1'))
  })

  it('shows the empty state', () => {
    feedsData.data = { feeds: [] }
    render(<FeedManager />)
    expect(screen.getByText('No feed subscriptions yet.')).toBeInTheDocument()
  })
})

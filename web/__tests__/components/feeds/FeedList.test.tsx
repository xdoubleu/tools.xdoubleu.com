import { render, screen } from '@testing-library/react'
import FeedList from '@/components/feeds/FeedList'

const feed = {
  id: 'feed-1',
  url: 'https://blog.example.com/feed.xml',
  title: 'Example Blog',
  lastFetchedAt: '2026-07-17T08:00:00Z',
  lastError: '',
  createdAt: '2026-07-01T08:00:00Z',
  sourceType: 'rss',
  inboundAddress: ''
}

describe('FeedList', () => {
  it('shows a loading state', () => {
    render(<FeedList feeds={[]} isLoading />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    render(<FeedList feeds={[]} error={new Error('nope')} />)
    expect(screen.getByText('Failed to load feeds.')).toBeInTheDocument()
  })

  it('shows an empty state', () => {
    render(<FeedList feeds={[]} />)
    expect(screen.getByText('No feed subscriptions yet.')).toBeInTheDocument()
  })

  it('lists feeds by title', () => {
    // @ts-expect-error -- partial Feed[] for test purposes
    render(<FeedList feeds={[feed]} />)
    expect(screen.getByText('Example Blog')).toBeInTheDocument()
    expect(screen.getByText(/Last fetched/)).toBeInTheDocument()
  })

  it('falls back to the URL when a feed has no title', () => {
    // @ts-expect-error -- partial Feed[] for test purposes
    render(<FeedList feeds={[{ ...feed, title: '' }]} />)
    expect(screen.getByText(feed.url)).toBeInTheDocument()
  })

  it('flags a feed whose last poll failed', () => {
    // @ts-expect-error -- partial Feed[] for test purposes
    render(<FeedList feeds={[{ ...feed, lastError: 'boom' }]} />)
    expect(screen.getByText('Last poll failed')).toBeInTheDocument()
  })

  it('shows no fetch status for a feed that has never been polled', () => {
    // @ts-expect-error -- partial Feed[] for test purposes
    render(<FeedList feeds={[{ ...feed, lastError: '', lastFetchedAt: '' }]} />)
    expect(screen.queryByText('Last poll failed')).not.toBeInTheDocument()
    expect(screen.queryByText(/Last fetched/)).not.toBeInTheDocument()
  })
})

import { render, screen } from '@testing-library/react'

const feedsData: { data?: unknown; error?: Error; isLoading: boolean } = {
  data: undefined,
  error: undefined,
  isLoading: false
}

jest.mock('@/hooks/useBookFeeds', () => ({
  useFeeds: () => feedsData
}))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Link
})

import SubscribedFeedsCard from '@/components/reading/SubscribedFeedsCard'

const feed = {
  id: 'feed-1',
  url: 'https://blog.example.com/feed.xml',
  title: 'Example Blog',
  koboSync: false,
  lastFetchedAt: '2026-07-17T08:00:00Z',
  lastError: '',
  createdAt: '2026-07-01T08:00:00Z'
}

describe('SubscribedFeedsCard', () => {
  beforeEach(() => {
    feedsData.data = { feeds: [feed] }
    feedsData.error = undefined
    feedsData.isLoading = false
  })

  it('lists subscribed feeds and links to settings', () => {
    render(<SubscribedFeedsCard />)
    expect(screen.getByText('Example Blog')).toBeInTheDocument()
    expect(screen.getByText('Manage').closest('a')).toHaveAttribute('href', '/reading/settings')
  })

  it('shows an empty state when there are no feeds', () => {
    feedsData.data = { feeds: [] }
    render(<SubscribedFeedsCard />)
    expect(screen.getByText('No feed subscriptions yet.')).toBeInTheDocument()
  })

  it('flags a feed whose last poll failed', () => {
    feedsData.data = { feeds: [{ ...feed, lastError: 'boom' }] }
    render(<SubscribedFeedsCard />)
    expect(screen.getByText('Last poll failed')).toBeInTheDocument()
  })

  it('falls back to the URL when a feed has no title', () => {
    feedsData.data = { feeds: [{ ...feed, title: '' }] }
    render(<SubscribedFeedsCard />)
    expect(screen.getByText(feed.url)).toBeInTheDocument()
  })

  it('shows no fetch status for a feed that has never been polled', () => {
    feedsData.data = { feeds: [{ ...feed, lastError: '', lastFetchedAt: '' }] }
    render(<SubscribedFeedsCard />)
    expect(screen.queryByText('Last poll failed')).not.toBeInTheDocument()
    expect(screen.queryByText(/Last fetched/)).not.toBeInTheDocument()
  })

  it('shows an error state', () => {
    feedsData.data = undefined
    feedsData.error = new Error('nope')
    render(<SubscribedFeedsCard />)
    expect(screen.getByText('Failed to load feeds.')).toBeInTheDocument()
  })
})

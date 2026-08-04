import { render, screen, fireEvent } from '@testing-library/react'

const feedsData: { data?: { feeds: unknown[] }; error?: Error; isLoading: boolean } = {
  data: undefined,
  error: undefined,
  isLoading: false
}

jest.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => feedsData
}))

jest.mock('@/components/feeds/FeedManager', () => () => <div data-testid="feed-manager" />)

import FeedsHeader from '@/components/feeds/FeedsHeader'

describe('FeedsHeader', () => {
  beforeEach(() => {
    feedsData.data = { feeds: [{ id: 'feed-1' }] }
    feedsData.error = undefined
    feedsData.isLoading = false
  })

  it('renders the page title', () => {
    render(<FeedsHeader />)
    expect(screen.getByRole('heading', { name: 'Feeds' })).toBeInTheDocument()
  })

  it('does not link back to /books', () => {
    render(<FeedsHeader />)
    expect(screen.queryByRole('link', { name: /books/i })).not.toBeInTheDocument()
  })

  it('links to the feed stats page', () => {
    render(<FeedsHeader />)
    expect(screen.getByRole('link', { name: 'Stats' })).toHaveAttribute('href', '/feeds/stats')
  })

  it('starts collapsed when the user already has feeds', () => {
    render(<FeedsHeader />)
    expect(screen.queryByTestId('feed-manager')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Manage feeds' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
  })

  it('expands to show FeedManager when the toggle is clicked', () => {
    render(<FeedsHeader />)
    fireEvent.click(screen.getByRole('button', { name: 'Manage feeds' }))
    expect(screen.getByTestId('feed-manager')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hide' })).toHaveAttribute('aria-expanded', 'true')
  })

  it('collapses again when the toggle is clicked a second time', () => {
    render(<FeedsHeader />)
    fireEvent.click(screen.getByRole('button', { name: 'Manage feeds' }))
    fireEvent.click(screen.getByRole('button', { name: 'Hide' }))
    expect(screen.queryByTestId('feed-manager')).not.toBeInTheDocument()
  })

  it('starts expanded when the user has no feeds yet', () => {
    feedsData.data = { feeds: [] }
    render(<FeedsHeader />)
    expect(screen.getByTestId('feed-manager')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hide' })).toHaveAttribute('aria-expanded', 'true')
  })
})

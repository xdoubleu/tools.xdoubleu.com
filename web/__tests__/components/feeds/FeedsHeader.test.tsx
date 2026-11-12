import { render, screen, fireEvent } from '@testing-library/react'

const feedsData: { data?: { feeds: unknown[] }; error?: Error; isLoading: boolean } = {
  data: undefined,
  error: undefined,
  isLoading: false
}

vi.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => feedsData
}))

const mockUseCurrentUser = vi.fn()
vi.mock('@/hooks/useAuth', () => ({
  useCurrentUser: () => mockUseCurrentUser()
}))

vi.mock('@/components/feeds/FeedManager', () => ({
  default: () => <div data-testid="feed-manager" />
}))

import FeedsHeader from '@/components/feeds/FeedsHeader'

describe('FeedsHeader', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseCurrentUser.mockReturnValue({ data: { role: 'user' } })
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

  const openMore = () => fireEvent.click(screen.getByRole('button', { name: 'More' }))

  it('links to the filtered and expired views from the More menu', () => {
    render(<FeedsHeader />)
    openMore()
    expect(screen.getByRole('menuitem', { name: 'Filtered' })).toHaveAttribute(
      'href',
      '/feeds/filtered'
    )
    expect(screen.getByRole('menuitem', { name: 'Expired' })).toHaveAttribute(
      'href',
      '/feeds/expired'
    )
  })

  it('links to the feed stats page', () => {
    render(<FeedsHeader />)
    openMore()
    expect(screen.getByRole('menuitem', { name: 'Stats' })).toHaveAttribute('href', '/feeds/stats')
  })

  it('links to the feed settings page', () => {
    render(<FeedsHeader />)
    openMore()
    expect(screen.getByRole('menuitem', { name: 'Settings' })).toHaveAttribute(
      'href',
      '/feeds/settings'
    )
  })

  it('hides the health link from a non-admin viewer', () => {
    render(<FeedsHeader />)
    openMore()
    expect(screen.queryByRole('menuitem', { name: 'Health' })).not.toBeInTheDocument()
  })

  it('links to the feed health page for an admin viewer', () => {
    mockUseCurrentUser.mockReturnValue({ data: { role: 'admin' } })
    render(<FeedsHeader />)
    openMore()
    expect(screen.getByRole('menuitem', { name: 'Health' })).toHaveAttribute(
      'href',
      '/feeds/health'
    )
  })

  it('starts closed when the user already has feeds', () => {
    render(<FeedsHeader />)
    expect(screen.queryByTestId('feed-manager')).not.toBeInTheDocument()
  })

  it('opens FeedManager in a dialog and closes it again', () => {
    render(<FeedsHeader />)
    fireEvent.click(screen.getByRole('button', { name: 'Manage feeds' }))
    expect(screen.getByRole('dialog', { name: 'Manage feeds' })).toBeInTheDocument()
    expect(screen.getByTestId('feed-manager')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(screen.queryByTestId('feed-manager')).not.toBeInTheDocument()
  })

  it('does not open the manager on load when there are no feeds', () => {
    feedsData.data = { feeds: [] }
    render(<FeedsHeader />)
    expect(screen.queryByTestId('feed-manager')).not.toBeInTheDocument()
  })
})

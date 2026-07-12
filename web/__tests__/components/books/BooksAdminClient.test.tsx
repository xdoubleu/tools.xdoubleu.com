import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

const mockUseResyncRefresh = jest.fn().mockReturnValue({
  connected: true,
  isRefreshing: false,
  lastRefresh: null,
  processed: null,
  total: null,
  quotaReached: false,
  refresh: jest.fn()
})

jest.mock('@/lib/books/resyncRefresh', () => ({
  useResyncRefresh: (onSynced?: () => void, force?: boolean) =>
    mockUseResyncRefresh(onSynced, force)
}))

const mockCancelResync = jest.fn()
jest.mock('@/hooks/useBooks', () => ({
  useCancelResync: () => mockCancelResync
}))

jest.mock('@/components/books/ResyncWizard', () => ({
  __esModule: true,
  default: () => <div data-testid="resync-wizard" />
}))

jest.mock('@/components/books/SourceStats', () => ({
  __esModule: true,
  default: () => <div data-testid="source-stats" />
}))

jest.mock('@/components/books/ManageDuplicatesDialog', () => ({
  __esModule: true,
  default: ({ open }: { open: boolean }) => (open ? <div data-testid="duplicates-dialog" /> : null)
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

import BooksAdminClient from '@/components/books/BooksAdminClient'

describe('BooksAdminClient', () => {
  it('renders the admin tools heading', () => {
    render(<BooksAdminClient />)
    expect(screen.getByRole('heading', { name: 'Books admin tools' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /books', () => {
    render(<BooksAdminClient />)
    expect(screen.getByRole('link', { name: 'Books' })).toHaveAttribute('href', '/books')
  })

  it('renders the scan section with a start button', () => {
    render(<BooksAdminClient />)
    expect(
      screen.getByRole('heading', { name: /scan for metadata differences/i })
    ).toBeInTheDocument()
    expect(screen.getByTestId('resync-openlibrary-btn')).toBeInTheDocument()
  })

  it('renders the wizard section with the ResyncWizard component', () => {
    render(<BooksAdminClient />)
    expect(screen.getByRole('heading', { name: /review flagged books/i })).toBeInTheDocument()
    expect(screen.getByTestId('resync-wizard')).toBeInTheDocument()
  })

  it('renders the Find duplicates section with button', () => {
    render(<BooksAdminClient />)
    expect(screen.getByRole('heading', { name: /find duplicates/i })).toBeInTheDocument()
    expect(screen.getByTestId('find-duplicates-btn')).toBeInTheDocument()
  })

  it('starts with the force checkbox unchecked and passes false', () => {
    render(<BooksAdminClient />)
    expect(screen.getByTestId('resync-force-checkbox')).not.toBeChecked()
    expect(mockUseResyncRefresh).toHaveBeenLastCalledWith(expect.any(Function), false)
  })

  it('toggling the checkbox passes force through to useResyncRefresh', () => {
    render(<BooksAdminClient />)
    fireEvent.click(screen.getByTestId('resync-force-checkbox'))
    expect(screen.getByTestId('resync-force-checkbox')).toBeChecked()
    expect(mockUseResyncRefresh).toHaveBeenLastCalledWith(expect.any(Function), true)
  })

  it('does not render a Stop button while idle', () => {
    render(<BooksAdminClient />)
    expect(screen.queryByTestId('resync-cancel-btn')).not.toBeInTheDocument()
  })

  it('renders a Stop button while refreshing and calls cancelResync on click', () => {
    mockUseResyncRefresh.mockReturnValueOnce({
      connected: true,
      isRefreshing: true,
      lastRefresh: null,
      processed: 10,
      total: 100,
      quotaReached: false,
      refresh: jest.fn()
    })
    render(<BooksAdminClient />)

    const stopBtn = screen.getByTestId('resync-cancel-btn')
    fireEvent.click(stopBtn)
    expect(mockCancelResync).toHaveBeenCalledTimes(1)
  })

  it('shows a quota-reached notice when the resync hook reports one', () => {
    mockUseResyncRefresh.mockReturnValueOnce({
      connected: true,
      isRefreshing: true,
      lastRefresh: null,
      processed: 500,
      total: 1000,
      quotaReached: true,
      refresh: jest.fn()
    })
    render(<BooksAdminClient />)
    expect(screen.getByTestId('resync-quota-reached')).toBeInTheDocument()
  })
})

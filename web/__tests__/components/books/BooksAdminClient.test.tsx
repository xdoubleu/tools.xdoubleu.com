import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/lib/books/resyncRefresh', () => ({
  useResyncRefresh: () => ({
    connected: true,
    isRefreshing: false,
    lastRefresh: null,
    processed: null,
    total: null,
    refresh: jest.fn()
  })
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
})

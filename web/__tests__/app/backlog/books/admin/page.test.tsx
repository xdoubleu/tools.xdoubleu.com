import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/lib/backlog/resyncRefresh', () => ({
  useResyncRefresh: () => ({
    connected: true,
    isRefreshing: false,
    lastRefresh: null,
    processed: null,
    total: null,
    refresh: jest.fn()
  })
}))

jest.mock('@/components/backlog/SelectiveResync', () => ({
  __esModule: true,
  default: () => <div data-testid="selective-resync" />
}))

jest.mock('@/components/backlog/ManageDuplicatesDialog', () => ({
  __esModule: true,
  default: ({ open }: { open: boolean }) => (open ? <div data-testid="duplicates-dialog" /> : null)
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

import BacklogBooksAdminPage from '@/app/backlog/books/admin/page'

describe('BacklogBooksAdminPage', () => {
  it('renders the admin tools heading', () => {
    render(<BacklogBooksAdminPage />)
    expect(screen.getByRole('heading', { name: 'Books admin tools' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /backlog/books', () => {
    render(<BacklogBooksAdminPage />)
    expect(screen.getByRole('link', { name: 'Books' })).toHaveAttribute('href', '/backlog/books')
  })

  it('renders the Resync all metadata section with button', () => {
    render(<BacklogBooksAdminPage />)
    expect(screen.getByRole('heading', { name: /resync all metadata/i })).toBeInTheDocument()
    expect(screen.getByTestId('resync-openlibrary-btn')).toBeInTheDocument()
  })

  it('renders the Selective resync section with the SelectiveResync component', () => {
    render(<BacklogBooksAdminPage />)
    expect(screen.getByRole('heading', { name: /selective resync/i })).toBeInTheDocument()
    expect(screen.getByTestId('selective-resync')).toBeInTheDocument()
  })

  it('renders the Find duplicates section with button', () => {
    render(<BacklogBooksAdminPage />)
    expect(screen.getByRole('heading', { name: /find duplicates/i })).toBeInTheDocument()
    expect(screen.getByTestId('find-duplicates-btn')).toBeInTheDocument()
  })
})

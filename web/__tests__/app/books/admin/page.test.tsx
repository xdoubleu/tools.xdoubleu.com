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

jest.mock('@/components/books/SelectiveResync', () => ({
  __esModule: true,
  default: () => <div data-testid="selective-resync" />
}))

jest.mock('@/components/books/ManageDuplicatesDialog', () => ({
  __esModule: true,
  default: ({ open }: { open: boolean }) => (open ? <div data-testid="duplicates-dialog" /> : null)
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

import BacklogBooksAdminPage from '@/app/books/admin/page'

describe('BacklogBooksAdminPage', () => {
  it('renders the admin tools heading', () => {
    render(<BacklogBooksAdminPage />)
    expect(screen.getByRole('heading', { name: 'Books admin tools' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /books', () => {
    render(<BacklogBooksAdminPage />)
    expect(screen.getByRole('link', { name: 'Books' })).toHaveAttribute('href', '/books')
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

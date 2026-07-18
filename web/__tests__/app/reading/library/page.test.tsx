import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/reading/BooksSection', () => () => <div data-testid="books-section" />)

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import BacklogBooksLibraryPage from '@/app/reading/library/page'

describe('BacklogBooksLibraryPage', () => {
  it('renders the Library heading', async () => {
    render(await BacklogBooksLibraryPage())
    expect(screen.getByRole('heading', { name: 'Library' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /reading', async () => {
    render(await BacklogBooksLibraryPage())
    expect(screen.getByRole('link', { name: 'Reading' })).toHaveAttribute('href', '/reading')
  })

  it('renders the BooksSection', async () => {
    render(await BacklogBooksLibraryPage())
    expect(screen.getByTestId('books-section')).toBeInTheDocument()
  })
})

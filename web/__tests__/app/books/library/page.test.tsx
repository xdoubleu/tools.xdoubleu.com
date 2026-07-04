import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/books/BooksSection', () => () => <div data-testid="books-section" />)

import BacklogBooksLibraryPage from '@/app/books/library/page'

describe('BacklogBooksLibraryPage', () => {
  it('renders the Library heading', () => {
    render(<BacklogBooksLibraryPage />)
    expect(screen.getByRole('heading', { name: 'Library' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /books', () => {
    render(<BacklogBooksLibraryPage />)
    expect(screen.getByRole('link', { name: 'Books' })).toHaveAttribute('href', '/books')
  })

  it('renders the BooksSection', () => {
    render(<BacklogBooksLibraryPage />)
    expect(screen.getByTestId('books-section')).toBeInTheDocument()
  })
})

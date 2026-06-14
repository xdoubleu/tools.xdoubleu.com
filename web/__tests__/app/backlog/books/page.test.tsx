import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/backlog/BooksDashboard', () => () => <div data-testid="books-dashboard" />)

import BacklogBooksPage from '@/app/backlog/books/page'

describe('BacklogBooksPage', () => {
  it('renders the Books heading', () => {
    render(<BacklogBooksPage />)
    expect(screen.getByRole('heading', { name: 'Books' })).toBeInTheDocument()
  })

  it('renders a settings link pointing to /backlog/settings', () => {
    render(<BacklogBooksPage />)
    const link = screen.getByRole('link', { name: /settings/i })
    expect(link).toHaveAttribute('href', '/backlog/books/settings')
  })

  it('renders the BooksDashboard', () => {
    render(<BacklogBooksPage />)
    expect(screen.getByTestId('books-dashboard')).toBeInTheDocument()
  })
})

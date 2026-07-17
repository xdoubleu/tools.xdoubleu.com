import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/reading/BooksDashboard', () => () => <div data-testid="books-dashboard" />)

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

import BacklogBooksPage from '@/app/reading/page'

describe('BacklogBooksPage', () => {
  it('renders the Books heading', async () => {
    render(await BacklogBooksPage())
    expect(screen.getByRole('heading', { name: 'Reading' })).toBeInTheDocument()
  })

  it('renders a settings link pointing to /backlog/settings', async () => {
    render(await BacklogBooksPage())
    const link = screen.getByRole('link', { name: /settings/i })
    expect(link).toHaveAttribute('href', '/reading/settings')
  })

  it('renders the BooksDashboard', async () => {
    render(await BacklogBooksPage())
    expect(screen.getByTestId('books-dashboard')).toBeInTheDocument()
  })
})

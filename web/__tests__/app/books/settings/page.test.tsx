import { render, screen } from '@testing-library/react'

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/books/BooksSettingsClient', () => () => (
  <div data-testid="books-settings-client" />
))

import BacklogBooksSettingsPage from '@/app/books/settings/page'

describe('BacklogBooksSettingsPage', () => {
  it('server-fetches kobo devices and renders the client component', async () => {
    render(await BacklogBooksSettingsPage())
    expect(screen.getByTestId('books-settings-client')).toBeInTheDocument()
  })
})

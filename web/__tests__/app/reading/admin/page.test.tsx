import { render, screen } from '@testing-library/react'

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/reading/BooksAdminClient', () => () => (
  <div data-testid="books-admin-client" />
))

import BacklogBooksAdminPage from '@/app/reading/admin/page'

describe('BacklogBooksAdminPage', () => {
  it('server-fetches the catalog and renders the client component', async () => {
    render(await BacklogBooksAdminPage())
    expect(screen.getByTestId('books-admin-client')).toBeInTheDocument()
  })
})

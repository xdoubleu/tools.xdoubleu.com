import React from 'react'
import { render, screen } from '@testing-library/react'

const fetchOrNull = jest.fn()

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => Promise<unknown>) => fetchOrNull(fn)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

jest.mock('@/components/recipes/CategoryManager', () => ({
  __esModule: true,
  default: () => <div data-testid="client" />
}))
jest.mock('@/components/recipes/ItemCatalog', () => ({
  __esModule: true,
  default: () => <div data-testid="item-catalog" />
}))
jest.mock('@/components/recipes/StoreManager', () => ({
  __esModule: true,
  default: () => <div data-testid="store-manager" />
}))

import Page from '@/app/shoppinglist/settings/page'

describe('ShoppingListSettingsPage', () => {
  it('renders with server-fetched data', async () => {
    fetchOrNull.mockResolvedValue({})
    render(await Page())
    expect(screen.getByTestId('client')).toBeInTheDocument()
  })

  it('renders when the server fetch returns null', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await Page())
    expect(screen.getByTestId('client')).toBeInTheDocument()
  })
})

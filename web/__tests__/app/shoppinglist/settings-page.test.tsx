import React from 'react'
import { render, screen } from '@testing-library/react'

const listCategories = jest.fn().mockResolvedValue({})
const listItemNames = jest.fn().mockResolvedValue({})
const listItemCategories = jest.fn().mockResolvedValue({})
const listStores = jest.fn().mockResolvedValue({})

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    listCategories: (...args: unknown[]) => listCategories(...args),
    listItemNames: (...args: unknown[]) => listItemNames(...args),
    listItemCategories: (...args: unknown[]) => listItemCategories(...args),
    listStores: (...args: unknown[]) => listStores(...args)
  }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => Promise<unknown>) => fn()
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
    render(await Page())
    expect(screen.getByTestId('client')).toBeInTheDocument()
    expect(listCategories).toHaveBeenCalledWith({})
  })

  it('renders when the server fetch returns null', async () => {
    listCategories.mockResolvedValueOnce(null)
    listItemNames.mockResolvedValueOnce(null)
    listItemCategories.mockResolvedValueOnce(null)
    listStores.mockResolvedValueOnce(null)
    render(await Page())
    expect(screen.getByTestId('client')).toBeInTheDocument()
  })
})

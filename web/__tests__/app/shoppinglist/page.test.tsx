import React from 'react'
import { render, screen } from '@testing-library/react'

const getCustomList = jest.fn().mockResolvedValue({})
const listCategories = jest.fn().mockResolvedValue({})

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    getCustomList: (...args: unknown[]) => getCustomList(...args),
    listCategories: (...args: unknown[]) => listCategories(...args)
  }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => Promise<unknown>) => fn()
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

jest.mock('@/components/shoppinglist/ShoppingListPageClient', () => ({
  __esModule: true,
  default: () => <div data-testid="client" />
}))

import Page from '@/app/shoppinglist/page'

describe('ShoppingListPage', () => {
  it('renders with server-fetched data', async () => {
    render(await Page())
    expect(screen.getByTestId('client')).toBeInTheDocument()
    expect(getCustomList).toHaveBeenCalledWith({})
    expect(listCategories).toHaveBeenCalledWith({})
  })

  it('renders when the server fetch returns null', async () => {
    getCustomList.mockResolvedValueOnce(null)
    listCategories.mockResolvedValueOnce(null)
    render(await Page())
    expect(screen.getByTestId('client')).toBeInTheDocument()
  })
})

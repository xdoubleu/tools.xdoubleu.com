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

jest.mock('@/app/recipes/[id]/edit/EditRecipeClient', () => ({
  __esModule: true,
  default: () => <div data-testid="client" />
}))

import Page from '@/app/recipes/[id]/edit/page'

describe('EditRecipePage', () => {
  it('renders with server-fetched data', async () => {
    fetchOrNull.mockResolvedValue({})
    render(await Page({ params: Promise.resolve({ id: 'r1' }) }))
    expect(screen.getByTestId('client')).toBeInTheDocument()
  })

  it('renders when the server fetch returns null', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await Page({ params: Promise.resolve({ id: 'r1' }) }))
    expect(screen.getByTestId('client')).toBeInTheDocument()
  })
})

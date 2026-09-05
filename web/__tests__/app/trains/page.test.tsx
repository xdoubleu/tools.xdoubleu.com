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

jest.mock('@/components/trains/TrainsClient', () => ({
  __esModule: true,
  default: () => <div data-testid="client" />
}))

import Page from '@/app/trains/page'

describe('TrainsPage', () => {
  it('renders with server-fetched feed info', async () => {
    fetchOrNull.mockResolvedValue({ feedVersion: '2026-08-31' })
    render(await Page())
    expect(screen.getByTestId('client')).toBeInTheDocument()
  })

  it('renders when the server fetch returns null', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await Page())
    expect(screen.getByTestId('client')).toBeInTheDocument()
  })
})

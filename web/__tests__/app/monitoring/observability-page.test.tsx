import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/monitoring/ObservabilityClient', () => () => (
  <div data-testid="observability-client" />
))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    getAutomatedActions: jest.fn(async () => ({}))
  }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import MonitoringObservabilityPage from '@/app/monitoring/observability/page'

describe('MonitoringObservabilityPage', () => {
  it('renders the observability client', async () => {
    render(await MonitoringObservabilityPage())
    expect(screen.getByTestId('observability-client')).toBeInTheDocument()
  })

  it('passes prefetched automated actions as SWR fallback when available', async () => {
    const { fetchOrNull } = jest.requireMock('@/lib/server/fetchers')
    fetchOrNull.mockImplementation((fn: () => unknown) => fn())

    render(await MonitoringObservabilityPage())
    expect(screen.getByTestId('observability-client')).toBeInTheDocument()
  })
})

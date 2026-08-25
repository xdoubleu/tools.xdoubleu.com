import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/monitoring/ObservabilityClient', () => () => (
  <div data-testid="observability-client" />
))

const mockClient = {
  getJobStats: jest.fn(async () => ({})),
  getUsageStats: jest.fn(async () => ({})),
  getStorageStats: jest.fn(async () => ({})),
  getDatabaseStats: jest.fn(async () => ({})),
  getHostMetrics: jest.fn(async () => ({}))
}

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => mockClient)
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async (fn: () => unknown) => fn())
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

  it('renders when prefetching returns nothing', async () => {
    const { fetchOrNull } = jest.requireMock('@/lib/server/fetchers')
    fetchOrNull.mockImplementationOnce(async () => null)

    render(await MonitoringObservabilityPage())
    expect(screen.getByTestId('observability-client')).toBeInTheDocument()
  })
})

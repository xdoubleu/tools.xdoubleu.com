import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/monitoring/MonitoringSettingsClient', () => () => (
  <div data-testid="monitoring-settings-client" />
))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    listOAuthConnections: jest.fn(async () => ({}))
  }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import MonitoringConnectionsPage from '@/app/monitoring/connections/page'

describe('MonitoringConnectionsPage', () => {
  it('renders the connections client', async () => {
    render(await MonitoringConnectionsPage())
    expect(screen.getByTestId('monitoring-settings-client')).toBeInTheDocument()
  })

  it('passes prefetched connections as SWR fallback when available', async () => {
    const { fetchOrNull } = jest.requireMock('@/lib/server/fetchers')
    fetchOrNull.mockImplementation((fn: () => unknown) => fn())

    render(await MonitoringConnectionsPage())
    expect(screen.getByTestId('monitoring-settings-client')).toBeInTheDocument()
  })
})

import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/monitoring/MonitoringSettingsClient', () => () => (
  <div data-testid="monitoring-settings-client" />
))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    getNotificationSettings: jest.fn(async () => ({})),
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

import MonitoringSettingsPage from '@/app/monitoring/settings/page'

describe('MonitoringSettingsPage', () => {
  it('renders the settings client', async () => {
    render(await MonitoringSettingsPage())
    expect(screen.getByTestId('monitoring-settings-client')).toBeInTheDocument()
  })

  it('links back to /monitoring', async () => {
    render(await MonitoringSettingsPage())
    expect(screen.getByRole('link', { name: 'Back to monitoring' })).toHaveAttribute(
      'href',
      '/monitoring'
    )
  })

  it('passes prefetched data as SWR fallback when available', async () => {
    const { fetchOrNull } = jest.requireMock('@/lib/server/fetchers')
    fetchOrNull.mockImplementation((fn: () => unknown) => fn())

    render(await MonitoringSettingsPage())
    expect(screen.getByTestId('monitoring-settings-client')).toBeInTheDocument()
  })
})

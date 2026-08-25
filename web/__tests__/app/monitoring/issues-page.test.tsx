import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/monitoring/IssuesClient', () => () => <div data-testid="issues-client" />)

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    getFailingPullRequests: jest.fn(async () => ({})),
    getWorkflowRuns: jest.fn(async () => ({})),
    getSecurityAlerts: jest.fn(async () => ({})),
    getSentryIssues: jest.fn(async () => ({}))
  }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import MonitoringIssuesPage from '@/app/monitoring/issues/page'

describe('MonitoringIssuesPage', () => {
  it('renders the issues client', async () => {
    render(await MonitoringIssuesPage())
    expect(screen.getByTestId('issues-client')).toBeInTheDocument()
  })

  it('passes prefetched data as SWR fallback when available', async () => {
    const { fetchOrNull } = jest.requireMock('@/lib/server/fetchers')
    fetchOrNull.mockImplementation((fn: () => unknown) => fn())

    render(await MonitoringIssuesPage())
    expect(screen.getByTestId('issues-client')).toBeInTheDocument()
  })
})

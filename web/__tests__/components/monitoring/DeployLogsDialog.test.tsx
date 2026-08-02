import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import DeployLogsDialog from '@/components/monitoring/DeployLogsDialog'

const mockGetDeployLogs = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useDeployLogs: () => mockGetDeployLogs
}))

jest.mock('@sentry/nextjs', () => ({
  captureException: jest.fn()
}))

import * as Sentry from '@sentry/nextjs'

const mockCaptureException = jest.mocked(Sentry.captureException)

beforeEach(() => {
  mockGetDeployLogs.mockReset()
  mockCaptureException.mockReset()
})

describe('DeployLogsDialog', () => {
  it('does not fetch or render content when closed', () => {
    render(<DeployLogsDialog deploymentId="d1" open={false} onOpenChange={jest.fn()} />)
    expect(mockGetDeployLogs).not.toHaveBeenCalled()
    expect(screen.queryByText('Deploy logs')).not.toBeInTheDocument()
  })

  it('fetches and renders per-component logs when opened', async () => {
    mockGetDeployLogs.mockResolvedValue({
      logs: [
        {
          component: 'api',
          logType: 'BUILD',
          content: 'building api\n',
          truncated: false,
          deploymentId: 'd1'
        },
        {
          component: 'web',
          logType: 'DEPLOY',
          content: 'deploying web\n',
          truncated: true,
          deploymentId: 'd1'
        }
      ]
    })

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    expect(mockGetDeployLogs).toHaveBeenCalledWith('d1')
    await waitFor(() => expect(screen.getByText('building api')).toBeInTheDocument())
    expect(screen.getByText('deploying web')).toBeInTheDocument()
    expect(screen.getByText('api')).toBeInTheDocument()
    expect(screen.getByText('web')).toBeInTheDocument()
    expect(screen.getByText('truncated')).toBeInTheDocument()
  })

  it('labels a log block that came from a different deployment', async () => {
    mockGetDeployLogs.mockResolvedValue({
      logs: [
        {
          component: 'api',
          logType: 'BUILD',
          content: 'build failed\n',
          truncated: false,
          deploymentId: 'd1'
        },
        {
          component: 'api',
          logType: 'RUN',
          content: 'still serving\n',
          truncated: false,
          deploymentId: 'd0'
        }
      ]
    })

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('deployment d0')).toBeInTheDocument())
    expect(screen.queryByText('deployment d1')).not.toBeInTheDocument()
  })

  it('shows an empty state when there are no logs yet', async () => {
    mockGetDeployLogs.mockResolvedValue({ logs: [] })

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('No logs available yet.')).toBeInTheDocument())
  })

  it('shows an error state and reports to Sentry when the fetch fails', async () => {
    const error = new Error('boom')
    mockGetDeployLogs.mockRejectedValue(error)

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('Failed to load deploy logs.')).toBeInTheDocument())
    expect(mockCaptureException).toHaveBeenCalledTimes(1)
    expect(mockCaptureException).toHaveBeenCalledWith(error)
  })
})

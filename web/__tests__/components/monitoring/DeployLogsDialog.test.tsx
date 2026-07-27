import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import DeployLogsDialog from '@/components/monitoring/DeployLogsDialog'

const mockGetDeployLogs = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useDeployLogs: () => mockGetDeployLogs
}))

beforeEach(() => {
  mockGetDeployLogs.mockReset()
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
        { component: 'api', logType: 'BUILD', content: 'building api\n', truncated: false },
        { component: 'web', logType: 'DEPLOY', content: 'deploying web\n', truncated: true }
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

  it('shows an empty state when there are no logs yet', async () => {
    mockGetDeployLogs.mockResolvedValue({ logs: [] })

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('No logs available yet.')).toBeInTheDocument())
  })

  it('shows an error state when the fetch fails', async () => {
    mockGetDeployLogs.mockRejectedValue(new Error('boom'))

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('Failed to load deploy logs.')).toBeInTheDocument())
  })
})

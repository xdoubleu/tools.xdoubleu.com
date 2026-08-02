import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import DeployLogsDialog from '@/components/monitoring/DeployLogsDialog'
import {
  DeployComponentLogSchema,
  type DeployComponentLog
} from '@/lib/gen/observability/v1/observability_pb'

const mockGetDeployLogs = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useDeployLogs: () => mockGetDeployLogs
}))

jest.mock('@sentry/nextjs', () => ({
  captureException: jest.fn()
}))

import * as Sentry from '@sentry/nextjs'

const mockCaptureException = jest.mocked(Sentry.captureException)

type LogInit = MessageInitShape<typeof DeployComponentLogSchema>

// asyncLogs turns a plain array into the async iterable GetDeployLogs now
// streams, so tests can drive DeployLogsDialog the same way the real
// ConnectRPC client does.
function asyncLogs(logs: LogInit[]): AsyncIterable<DeployComponentLog> {
  return {
    async *[Symbol.asyncIterator]() {
      for (const log of logs) {
        yield create(DeployComponentLogSchema, log)
      }
    }
  }
}

// asyncLogsThatThrow yields the given logs, then rejects — matching a
// stream that fails partway through.
function asyncLogsThatThrow(logs: LogInit[], error: Error): AsyncIterable<DeployComponentLog> {
  return {
    async *[Symbol.asyncIterator]() {
      for (const log of logs) {
        yield create(DeployComponentLogSchema, log)
      }
      throw error
    }
  }
}

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
    mockGetDeployLogs.mockReturnValue(
      asyncLogs([
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
      ])
    )

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    expect(mockGetDeployLogs).toHaveBeenCalledWith('d1')
    await waitFor(() => expect(screen.getByText('building api')).toBeInTheDocument())
    expect(screen.getByText('deploying web')).toBeInTheDocument()
    expect(screen.getByText('api')).toBeInTheDocument()
    expect(screen.getByText('web')).toBeInTheDocument()
    expect(screen.getByText('truncated')).toBeInTheDocument()
  })

  it('labels a log block that came from a different deployment', async () => {
    mockGetDeployLogs.mockReturnValue(
      asyncLogs([
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
      ])
    )

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('deployment d0')).toBeInTheDocument())
    expect(screen.queryByText('deployment d1')).not.toBeInTheDocument()
  })

  it('shows an empty state when there are no logs yet', async () => {
    mockGetDeployLogs.mockReturnValue(asyncLogs([]))

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('No logs available yet.')).toBeInTheDocument())
  })

  it('shows an error state and reports to Sentry when the stream fails', async () => {
    const error = new Error('boom')
    mockGetDeployLogs.mockReturnValue(asyncLogsThatThrow([], error))

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('Failed to load deploy logs.')).toBeInTheDocument())
    expect(mockCaptureException).toHaveBeenCalledTimes(1)
    expect(mockCaptureException).toHaveBeenCalledWith(error)
  })

  it('renders logs that already streamed in before the stream later fails', async () => {
    const error = new Error('boom')
    mockGetDeployLogs.mockReturnValue(
      asyncLogsThatThrow(
        [
          {
            component: 'api',
            logType: 'BUILD',
            content: 'building api\n',
            truncated: false,
            deploymentId: 'd1'
          }
        ],
        error
      )
    )

    render(<DeployLogsDialog deploymentId="d1" open={true} onOpenChange={jest.fn()} />)

    await waitFor(() => expect(screen.getByText('Failed to load deploy logs.')).toBeInTheDocument())
    expect(mockCaptureException).toHaveBeenCalledWith(error)
  })
})

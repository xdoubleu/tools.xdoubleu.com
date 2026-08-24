import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { GetLogsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import LogsCard from '@/components/monitoring/LogsCard'

const mockUseLogs = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useLogs: (source: string, minLevel: string) => mockUseLogs(source, minLevel)
}))

beforeEach(() => {
  mockUseLogs.mockReset()
})

describe('LogsCard', () => {
  it('renders recent log entries', () => {
    mockUseLogs.mockReturnValue({
      data: create(GetLogsResponseSchema, {
        entries: [
          {
            occurredAt: '2026-01-01T00:00:00Z',
            source: 'api',
            level: 'error',
            message: 'boom',
            attrsJson: ''
          }
        ]
      }),
      isLoading: false
    })

    render(<LogsCard />)
    expect(screen.getByText('boom')).toBeInTheDocument()
    expect(screen.getAllByText('error').length).toBeGreaterThan(0)
    expect(screen.getAllByText('api').length).toBeGreaterThan(0)
  })

  it('renders warn, debug, and default levels with attrs', () => {
    mockUseLogs.mockReturnValue({
      data: create(GetLogsResponseSchema, {
        entries: [
          {
            occurredAt: '2026-01-01T00:00:00Z',
            source: 'web',
            level: 'warn',
            message: 'careful',
            attrsJson: '{"foo":"bar"}'
          },
          {
            occurredAt: '2026-01-01T00:00:01Z',
            source: 'web',
            level: 'debug',
            message: 'details',
            attrsJson: ''
          },
          {
            occurredAt: '2026-01-01T00:00:02Z',
            source: 'web',
            level: '',
            message: 'plain info',
            attrsJson: ''
          }
        ]
      }),
      isLoading: false
    })

    render(<LogsCard />)
    expect(screen.getByText('careful')).toBeInTheDocument()
    expect(screen.getByText('{"foo":"bar"}')).toBeInTheDocument()
    expect(screen.getByText('details')).toBeInTheDocument()
    expect(screen.getByText('plain info')).toBeInTheDocument()
  })

  it('re-fetches logs when the minimum level filter changes', async () => {
    mockUseLogs.mockReturnValue({
      data: create(GetLogsResponseSchema, { entries: [] }),
      isLoading: false
    })
    render(<LogsCard />)

    fireEvent.change(screen.getByLabelText('Minimum level'), { target: { value: 'warn' } })

    await waitFor(() => expect(mockUseLogs).toHaveBeenCalledWith('', 'warn'))
  })

  it('shows a loading state', () => {
    mockUseLogs.mockReturnValue({ data: undefined, isLoading: true })
    render(<LogsCard />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an empty state when there are no logs', () => {
    mockUseLogs.mockReturnValue({
      data: create(GetLogsResponseSchema, { entries: [] }),
      isLoading: false
    })
    render(<LogsCard />)
    expect(screen.getByText('No logs found.')).toBeInTheDocument()
  })

  it('re-fetches logs when the source filter changes', async () => {
    mockUseLogs.mockReturnValue({
      data: create(GetLogsResponseSchema, { entries: [] }),
      isLoading: false
    })
    render(<LogsCard />)

    fireEvent.change(screen.getByLabelText('Source'), { target: { value: 'web' } })

    await waitFor(() => expect(mockUseLogs).toHaveBeenCalledWith('web', ''))
  })
})

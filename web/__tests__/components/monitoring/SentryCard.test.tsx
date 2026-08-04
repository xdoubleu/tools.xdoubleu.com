import React from 'react'
import { create } from '@bufbuild/protobuf'
import { ConnectError, Code } from '@connectrpc/connect'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { GetSentryIssuesResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import SentryCard from '@/components/monitoring/SentryCard'

const mockResolveSentryIssue = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useResolveSentryIssue: () => mockResolveSentryIssue
}))

describe('SentryCard', () => {
  beforeEach(() => {
    mockResolveSentryIssue.mockReset()
    mockResolveSentryIssue.mockResolvedValue(undefined)
  })

  it('shows a loading state without data', () => {
    render(<SentryCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows a not-configured message', () => {
    const data = create(GetSentryIssuesResponseSchema, {
      configured: false,
      issues: [],
      unresolvedCount: 0
    })
    render(<SentryCard data={data} />)
    expect(screen.getByText('Sentry is not configured.')).toBeInTheDocument()
  })

  it('tags each merged issue with its project', () => {
    const data = create(GetSentryIssuesResponseSchema, {
      configured: true,
      unresolvedCount: 2,
      issues: [
        {
          id: '1',
          title: 'Boom A',
          culprit: '',
          permalink: 'https://s/1',
          count: 3n,
          lastSeen: '2026-07-10T00:00:00Z',
          level: 'error',
          project: 'proj-a'
        },
        {
          id: '2',
          title: 'Boom B',
          culprit: '',
          permalink: 'https://s/2',
          count: 1n,
          lastSeen: '2026-07-09T00:00:00Z',
          level: 'warning',
          project: 'proj-b'
        }
      ]
    })

    render(<SentryCard data={data} />)
    expect(screen.getByText('Boom A')).toBeInTheDocument()
    expect(screen.getByText('proj-a')).toBeInTheDocument()
    expect(screen.getByText('Boom B')).toBeInTheDocument()
    expect(screen.getByText('proj-b')).toBeInTheDocument()
  })

  it('resolves an issue when its Resolve button is clicked', async () => {
    const data = create(GetSentryIssuesResponseSchema, {
      configured: true,
      unresolvedCount: 1,
      issues: [
        {
          id: '1',
          title: 'Boom A',
          culprit: '',
          permalink: 'https://s/1',
          count: 3n,
          lastSeen: '2026-07-10T00:00:00Z',
          level: 'error',
          project: 'proj-a'
        }
      ]
    })

    render(<SentryCard data={data} />)
    fireEvent.click(screen.getByRole('button', { name: 'Resolve' }))

    await waitFor(() => expect(mockResolveSentryIssue).toHaveBeenCalledWith('1'))
  })

  it('shows a reconnect popup when resolving fails with Unauthenticated', async () => {
    mockResolveSentryIssue.mockRejectedValue(new ConnectError('needs reauth', Code.Unauthenticated))
    const data = create(GetSentryIssuesResponseSchema, {
      configured: true,
      unresolvedCount: 1,
      issues: [
        {
          id: '1',
          title: 'Boom A',
          culprit: '',
          permalink: 'https://s/1',
          count: 3n,
          lastSeen: '2026-07-10T00:00:00Z',
          level: 'error',
          project: 'proj-a'
        }
      ]
    })

    render(<SentryCard data={data} />)
    fireEvent.click(screen.getByRole('button', { name: 'Resolve' }))

    expect(await screen.findByText('Sentry needs to be reconnected')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Reconnect Sentry' })).toBeInTheDocument()
  })

  it('does not show the reconnect popup for other resolve failures', async () => {
    mockResolveSentryIssue.mockRejectedValue(new ConnectError('boom', Code.Internal))
    const data = create(GetSentryIssuesResponseSchema, {
      configured: true,
      unresolvedCount: 1,
      issues: [
        {
          id: '1',
          title: 'Boom A',
          culprit: '',
          permalink: 'https://s/1',
          count: 3n,
          lastSeen: '2026-07-10T00:00:00Z',
          level: 'error',
          project: 'proj-a'
        }
      ]
    })

    render(<SentryCard data={data} />)
    fireEvent.click(screen.getByRole('button', { name: 'Resolve' }))

    await waitFor(() => expect(mockResolveSentryIssue).toHaveBeenCalledWith('1'))
    expect(screen.queryByText('Sentry needs to be reconnected')).not.toBeInTheDocument()
  })
})

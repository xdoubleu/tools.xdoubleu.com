import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetSentryIssuesResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import SentryCard from '@/components/monitoring/SentryCard'

describe('SentryCard', () => {
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
})

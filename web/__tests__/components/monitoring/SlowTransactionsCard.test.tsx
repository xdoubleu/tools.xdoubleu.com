import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetSlowTransactionsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import SlowTransactionsCard from '@/components/monitoring/SlowTransactionsCard'

describe('SlowTransactionsCard', () => {
  it('shows an empty state without data', () => {
    render(<SlowTransactionsCard data={undefined} />)
    expect(screen.getByText('No transactions recorded yet.')).toBeInTheDocument()
  })

  it('shows a not-configured message when Sentry is unconfigured', () => {
    const data = create(GetSlowTransactionsResponseSchema, {
      configured: false,
      current: [],
      trending: []
    })
    render(<SlowTransactionsCard data={data} />)
    expect(screen.getByText('Sentry is not configured.')).toBeInTheDocument()
  })

  it('renders current transactions with formatted duration and requests', () => {
    const data = create(GetSlowTransactionsResponseSchema, {
      configured: true,
      current: [
        {
          transaction: 'GET /api/books',
          project: 'proj',
          p95DurationMs: 1234,
          requestCount: 42n
        }
      ],
      trending: []
    })
    render(<SlowTransactionsCard data={data} />)
    expect(screen.getByText('GET /api/books')).toBeInTheDocument()
    expect(screen.getByText('proj')).toBeInTheDocument()
    expect(screen.getByText('1.2 s')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.queryByText('Getting slower')).not.toBeInTheDocument()
  })

  it('renders trending regressions even when Sentry is unconfigured', () => {
    const data = create(GetSlowTransactionsResponseSchema, {
      configured: false,
      current: [],
      trending: [
        {
          transaction: 'GET /api/regressed',
          project: 'proj',
          priorAvgP95Ms: 100,
          recentAvgP95Ms: 250,
          pctChange: 1.5
        }
      ]
    })
    render(<SlowTransactionsCard data={data} />)
    expect(screen.getByText('Sentry is not configured.')).toBeInTheDocument()
    expect(screen.getByText('Getting slower')).toBeInTheDocument()
    expect(screen.getByText('GET /api/regressed')).toBeInTheDocument()
    expect(screen.getByText('+150%')).toBeInTheDocument()
    expect(screen.getByText('100 ms → 250 ms')).toBeInTheDocument()
    expect(screen.getByText('+150%').className).toContain('text-danger')
  })

  it('tones a trending regression as warn, not danger, below the danger threshold', () => {
    const data = create(GetSlowTransactionsResponseSchema, {
      configured: true,
      current: [],
      trending: [
        {
          transaction: 'GET /api/mild',
          project: 'proj',
          priorAvgP95Ms: 100,
          recentAvgP95Ms: 130,
          pctChange: 0.3
        }
      ]
    })
    render(<SlowTransactionsCard data={data} />)
    expect(screen.getByText('+30%')).toBeInTheDocument()
    expect(screen.getByText('+30%').className).toContain('text-warn')
  })
})

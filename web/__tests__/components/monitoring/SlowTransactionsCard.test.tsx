import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import {
  GetSlowTransactionsResponseSchema,
  GetAlertStatesResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
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

  describe('filtered', () => {
    const alertStates = create(GetAlertStatesResponseSchema, {
      states: [
        {
          ruleKey: 'slow_transaction_http_high',
          breaching: true,
          currentValue: 6000,
          threshold: 5000
        }
      ]
    })

    it('shows only rows breaching their class threshold and omits the trending section', () => {
      const data = create(GetSlowTransactionsResponseSchema, {
        configured: true,
        current: [
          { transaction: 'GET /fast', project: 'proj', p95DurationMs: 500, requestCount: 10n },
          { transaction: 'GET /slow', project: 'proj', p95DurationMs: 6000, requestCount: 5n }
        ],
        trending: [
          {
            transaction: 'GET /regressed',
            project: 'proj',
            priorAvgP95Ms: 100,
            recentAvgP95Ms: 250,
            pctChange: 1.5
          }
        ]
      })
      render(<SlowTransactionsCard data={data} alertStates={alertStates} filtered />)
      expect(screen.getByText('GET /slow')).toBeInTheDocument()
      expect(screen.queryByText('GET /fast')).not.toBeInTheDocument()
      expect(screen.queryByText('Getting slower')).not.toBeInTheDocument()
    })

    it('shows a threshold-specific empty state when nothing is over threshold', () => {
      const data = create(GetSlowTransactionsResponseSchema, {
        configured: true,
        current: [
          { transaction: 'GET /fast', project: 'proj', p95DurationMs: 500, requestCount: 10n }
        ],
        trending: []
      })
      render(<SlowTransactionsCard data={data} alertStates={alertStates} filtered />)
      expect(screen.getByText('No transactions currently over threshold.')).toBeInTheDocument()
    })

    it('never shows the Slow badge in filtered mode', () => {
      const data = create(GetSlowTransactionsResponseSchema, {
        configured: true,
        current: [
          { transaction: 'GET /slow', project: 'proj', p95DurationMs: 6000, requestCount: 5n }
        ],
        trending: []
      })
      render(<SlowTransactionsCard data={data} alertStates={alertStates} filtered />)
      expect(screen.getByText('GET /slow')).toBeInTheDocument()
      expect(screen.queryByText('Slow')).not.toBeInTheDocument()
    })
  })

  it('badges a current transaction that breaches its class threshold', () => {
    const data = create(GetSlowTransactionsResponseSchema, {
      configured: true,
      current: [
        {
          transaction: 'GET /games/api/progress',
          project: 'tools-api',
          p95DurationMs: 144000,
          requestCount: 35n
        }
      ],
      trending: []
    })
    const alertStates = create(GetAlertStatesResponseSchema, {
      states: [
        {
          ruleKey: 'slow_transaction_http_high',
          breaching: true,
          currentValue: 144000,
          threshold: 5000
        }
      ]
    })
    render(<SlowTransactionsCard data={data} alertStates={alertStates} />)
    expect(screen.getByText('Slow')).toBeInTheDocument()
  })

  it('does not badge a current transaction under its class threshold', () => {
    const data = create(GetSlowTransactionsResponseSchema, {
      configured: true,
      current: [
        {
          transaction: 'steam',
          project: 'tools-api',
          p95DurationMs: 24000,
          requestCount: 2n
        }
      ],
      trending: []
    })
    const alertStates = create(GetAlertStatesResponseSchema, {
      states: [
        {
          ruleKey: 'slow_transaction_job_high',
          breaching: false,
          currentValue: 24000,
          threshold: 60000
        }
      ]
    })
    render(<SlowTransactionsCard data={data} alertStates={alertStates} />)
    expect(screen.queryByText('Slow')).not.toBeInTheDocument()
  })

  it('does not badge anything when no alert states are loaded yet', () => {
    const data = create(GetSlowTransactionsResponseSchema, {
      configured: true,
      current: [
        {
          transaction: 'GET /games/api/progress',
          project: 'tools-api',
          p95DurationMs: 144000,
          requestCount: 35n
        }
      ],
      trending: []
    })
    render(<SlowTransactionsCard data={data} alertStates={undefined} />)
    expect(screen.queryByText('Slow')).not.toBeInTheDocument()
  })
})

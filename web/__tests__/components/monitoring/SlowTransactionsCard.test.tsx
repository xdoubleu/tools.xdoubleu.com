import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import {
  GetSlowTransactionsResponseSchema,
  GetAlertStatesResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
import SlowTransactionsCard, {
  toSpanBars,
  regressionDangerThreshold
} from '@/components/monitoring/SlowTransactionsCard'

// recharts needs a non-zero layout size that jsdom does not provide.
jest.mock('recharts', () => {
  const Original = jest.requireActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({
      children
    }: {
      children: React.ReactElement<{ width?: number; height?: number }>
    }) => <div>{React.cloneElement(children, { width: 600, height: 300 })}</div>
  }
})

describe('toSpanBars', () => {
  it('pivots current rows into sorted, slow-flagged bars', () => {
    const bars = toSpanBars(
      [
        {
          $typeName: 'observability.v1.SlowTransaction',
          transaction: 'GET /fast',
          project: 'proj',
          p95DurationMs: 500,
          requestCount: 10n
        },
        {
          $typeName: 'observability.v1.SlowTransaction',
          transaction: 'GET /slow',
          project: 'proj',
          p95DurationMs: 6000,
          requestCount: 5n
        }
      ],
      { slow_transaction_http_high: 5000 }
    )
    expect(bars).toEqual([
      { key: 'proj-GET /slow', label: 'proj · GET /slow', p95: 6000, requests: 5, slow: true },
      { key: 'proj-GET /fast', label: 'proj · GET /fast', p95: 500, requests: 10, slow: false }
    ])
  })
})

describe('SlowTransactionsCard', () => {
  it('shows an empty state without data', () => {
    render(<SlowTransactionsCard data={undefined} />)
    expect(screen.getByText('No spans recorded yet.')).toBeInTheDocument()
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

  it('plots current spans with a labeled bar and no table', () => {
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
    expect(screen.getByText('proj · GET /api/books')).toBeInTheDocument()
    expect(screen.getByText('1.2 s')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.queryByText('Getting slower')).not.toBeInTheDocument()
  })

  it('shows the over-threshold legend only when a bar is slow and unfiltered', () => {
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
    const data = create(GetSlowTransactionsResponseSchema, {
      configured: true,
      current: [
        { transaction: 'GET /slow', project: 'proj', p95DurationMs: 6000, requestCount: 5n }
      ],
      trending: []
    })
    render(<SlowTransactionsCard data={data} alertStates={alertStates} />)
    expect(screen.getByText('Over its class threshold')).toBeInTheDocument()
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
    expect(regressionDangerThreshold).toBe(0.5)
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
      expect(screen.getByText('proj · GET /slow')).toBeInTheDocument()
      expect(screen.queryByText('proj · GET /fast')).not.toBeInTheDocument()
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
      expect(screen.getByText('No spans currently over threshold.')).toBeInTheDocument()
    })

    it('never shows the over-threshold legend in filtered mode', () => {
      const data = create(GetSlowTransactionsResponseSchema, {
        configured: true,
        current: [
          { transaction: 'GET /slow', project: 'proj', p95DurationMs: 6000, requestCount: 5n }
        ],
        trending: []
      })
      render(<SlowTransactionsCard data={data} alertStates={alertStates} filtered />)
      expect(screen.getByText('proj · GET /slow')).toBeInTheDocument()
      expect(screen.queryByText('Over its class threshold')).not.toBeInTheDocument()
    })
  })

  it('does not show the over-threshold legend when nothing breaches its class threshold', () => {
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
    expect(screen.queryByText('Over its class threshold')).not.toBeInTheDocument()
  })

  it('does not flag anything as slow when no alert states are loaded yet', () => {
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
    expect(screen.queryByText('Over its class threshold')).not.toBeInTheDocument()
  })
})

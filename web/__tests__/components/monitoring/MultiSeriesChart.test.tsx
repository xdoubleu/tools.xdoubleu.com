import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import MultiSeriesChart, {
  pivotSeries,
  defaultSelection,
  xAxisTickFormatter,
  tooltipLabelFormatter,
  logSafeDomain,
  logAxisTickFormatter,
  type SeriesPoint,
  type SeriesMeta
} from '@/components/monitoring/MultiSeriesChart'

jest.mock('recharts', () => ({
  LineChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="line-chart">{children}</div>
  ),
  Line: ({ dataKey }: { dataKey: string }) => <div data-testid={`line-${dataKey}`} />,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>
}))

const points: SeriesPoint[] = [
  { day: '2026-01-01', seriesKey: 'a', value: 100 },
  { day: '2026-01-02', seriesKey: 'a', value: 150 },
  { day: '2026-01-01', seriesKey: 'b', value: 50 },
  { day: '2026-01-02', seriesKey: 'b', value: 60 }
]
const meta: SeriesMeta[] = [
  { key: 'a', label: 'proj · GET /a' },
  { key: 'b', label: 'proj · GET /b' }
]

describe('pivotSeries', () => {
  it('groups points by day and sorts ascending', () => {
    const rows = pivotSeries(points)
    expect(rows).toEqual([
      { day: '2026-01-01', a: 100, b: 50 },
      { day: '2026-01-02', a: 150, b: 60 }
    ])
  })

  it('returns an empty array for no points', () => {
    expect(pivotSeries([])).toEqual([])
  })
})

describe('defaultSelection', () => {
  it('picks the top N series by latest value', () => {
    expect(defaultSelection(points, meta, 1)).toEqual(['a'])
  })

  it('falls back to 0 for a series with no points', () => {
    const metaWithExtra = [...meta, { key: 'c', label: 'proj · GET /c' }]
    expect(defaultSelection(points, metaWithExtra, 3)).toEqual(['a', 'b', 'c'])
  })
})

describe('xAxisTickFormatter / tooltipLabelFormatter', () => {
  it('formats a YYYY-MM-DD day without a timezone shift', () => {
    expect(xAxisTickFormatter('2026-01-15')).toBe('15/01/2026')
    expect(tooltipLabelFormatter('2026-01-15')).toBe('15/01/2026')
  })

  it('falls back to the raw value when unparseable', () => {
    expect(xAxisTickFormatter('')).toBe('')
  })

  it('returns an empty string for a non-string tooltip label', () => {
    expect(tooltipLabelFormatter(42)).toBe('')
  })
})

describe('logSafeDomain', () => {
  it('returns the min/max of positive values', () => {
    expect(logSafeDomain(points)).toEqual([50, 150])
  })

  it('defaults to [1, 1] when there are no positive values', () => {
    expect(logSafeDomain([{ day: '2026-01-01', seriesKey: 'a', value: 0 }])).toEqual([1, 1])
  })
})

describe('logAxisTickFormatter', () => {
  it('abbreviates values at or above 1000', () => {
    expect(logAxisTickFormatter(144000)).toBe('144k')
  })

  it('rounds values below 1000', () => {
    expect(logAxisTickFormatter(42.6)).toBe('43')
  })
})

describe('MultiSeriesChart', () => {
  const valueFormatter = (v: number) => `${v}ms`

  it('renders lines for the default selection', () => {
    render(
      <MultiSeriesChart
        points={points}
        meta={meta}
        valueLabel="p95 duration"
        valueFormatter={valueFormatter}
        defaultSelectionCount={1}
      />
    )
    expect(screen.getByTestId('line-a')).toBeInTheDocument()
    expect(screen.queryByTestId('line-b')).not.toBeInTheDocument()
  })

  it('toggles a series on and off via its checkbox', () => {
    render(
      <MultiSeriesChart
        points={points}
        meta={meta}
        valueLabel="p95 duration"
        valueFormatter={valueFormatter}
        defaultSelectionCount={1}
      />
    )
    fireEvent.click(screen.getByLabelText('proj · GET /b'))
    expect(screen.getByTestId('line-b')).toBeInTheDocument()

    fireEvent.click(screen.getByLabelText('proj · GET /a'))
    expect(screen.queryByTestId('line-a')).not.toBeInTheDocument()
  })

  it('filters the series list via the search box', () => {
    render(
      <MultiSeriesChart
        points={points}
        meta={meta}
        valueLabel="p95 duration"
        valueFormatter={valueFormatter}
      />
    )
    fireEvent.change(screen.getByPlaceholderText('Filter series…'), {
      target: { value: 'GET /a' }
    })
    expect(screen.getByLabelText('proj · GET /a')).toBeInTheDocument()
    expect(screen.queryByLabelText('proj · GET /b')).not.toBeInTheDocument()
  })

  it('shows a message when the search filter matches nothing', () => {
    render(
      <MultiSeriesChart
        points={points}
        meta={meta}
        valueLabel="p95 duration"
        valueFormatter={valueFormatter}
      />
    )
    fireEvent.change(screen.getByPlaceholderText('Filter series…'), {
      target: { value: 'nothing matches this' }
    })
    expect(screen.getByText('No matching series.')).toBeInTheDocument()
  })

  it('toggles the y-axis to a log scale', () => {
    render(
      <MultiSeriesChart
        points={points}
        meta={meta}
        valueLabel="p95 duration"
        valueFormatter={valueFormatter}
      />
    )
    const toggle = screen.getByLabelText('Log scale')
    expect(toggle).not.toBeChecked()
    fireEvent.click(toggle)
    expect(toggle).toBeChecked()
  })

  it('shows a placeholder when no series are selected', () => {
    render(
      <MultiSeriesChart
        points={points}
        meta={meta}
        valueLabel="p95 duration"
        valueFormatter={valueFormatter}
        defaultSelectionCount={1}
      />
    )
    fireEvent.click(screen.getByLabelText('proj · GET /a'))
    expect(screen.getByText('Select at least one series to plot.')).toBeInTheDocument()
  })

  it('disables unselected checkboxes once the palette cap is reached', () => {
    const manyMeta: SeriesMeta[] = Array.from({ length: 8 }, (_, i) => ({
      key: `s${i}`,
      label: `series ${i}`
    }))
    const manyPoints: SeriesPoint[] = manyMeta.map((m, i) => ({
      day: '2026-01-01',
      seriesKey: m.key,
      value: i
    }))
    render(
      <MultiSeriesChart
        points={manyPoints}
        meta={manyMeta}
        valueLabel="p95 duration"
        valueFormatter={valueFormatter}
        defaultSelectionCount={7}
      />
    )
    expect(screen.getByLabelText('series 0')).toBeDisabled()
  })
})

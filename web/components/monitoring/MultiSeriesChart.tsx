'use client'

import { useMemo, useState } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { CATEGORICAL_PALETTE, chartTooltipStyle } from '@/lib/observability'
import { formatDate } from '@/lib/dates'

// SeriesPoint is one (day, series) sample — the flat shape a caller pivots
// from its own API response before handing it to MultiSeriesChart. Not
// transaction-specific: any per-entity daily metric fits this shape.
export interface SeriesPoint {
  day: string // YYYY-MM-DD
  seriesKey: string
  value: number
}

export interface SeriesMeta {
  key: string
  label: string
}

// pivotSeries turns flat {day, seriesKey, value}[] rows into
// {day, [seriesKey]: value}[] rows — the shape recharts needs to draw one
// <Line> per series inside a single <LineChart>.
export function pivotSeries(points: SeriesPoint[]): Record<string, number | string>[] {
  const byDay = new Map<string, Record<string, number | string>>()
  for (const p of points) {
    let row = byDay.get(p.day)
    if (!row) {
      row = { day: p.day }
      byDay.set(p.day, row)
    }
    row[p.seriesKey] = p.value
  }
  return [...byDay.values()].sort((a, b) => String(a.day).localeCompare(String(b.day)))
}

// defaultSelection picks the `count` series with the highest latest value —
// not empty, not all — so the chart is useful without any interaction.
export function defaultSelection(
  points: SeriesPoint[],
  meta: SeriesMeta[],
  count: number
): string[] {
  const latestByKey = new Map<string, { day: string; value: number }>()
  for (const p of points) {
    const cur = latestByKey.get(p.seriesKey)
    if (!cur || p.day > cur.day) {
      latestByKey.set(p.seriesKey, { day: p.day, value: p.value })
    }
  }
  return meta
    .map((m) => ({ key: m.key, value: latestByKey.get(m.key)?.value ?? 0 }))
    .sort((a, b) => b.value - a.value)
    .slice(0, count)
    .map((s) => s.key)
}

export function xAxisTickFormatter(value: string): string {
  return formatDate(value) || value
}

export function tooltipLabelFormatter(value: unknown): string {
  return formatDate(typeof value === 'string' ? value : '')
}

// logSafeDomain returns a [min, max] domain safe for a log-scale y-axis —
// recharts' log scale breaks on a zero/negative lower bound, so the floor is
// clamped to the smallest positive value seen (or 1 if there isn't one).
export function logSafeDomain(points: SeriesPoint[]): [number, number] {
  const positive = points.map((p) => p.value).filter((v) => v > 0)
  if (positive.length === 0) return [1, 1]
  return [Math.min(...positive), Math.max(...positive)]
}

export function logAxisTickFormatter(value: number): string {
  return value >= 1000 ? `${Math.round(value / 1000)}k` : `${Math.round(value)}`
}

export interface MultiSeriesChartProps {
  points: SeriesPoint[]
  meta: SeriesMeta[]
  valueLabel: string
  valueFormatter: (v: number) => string
  searchPlaceholder?: string
  defaultSelectionCount?: number
}

export default function MultiSeriesChart({
  points,
  meta,
  valueLabel,
  valueFormatter,
  searchPlaceholder = 'Filter series…',
  defaultSelectionCount = 5
}: MultiSeriesChartProps) {
  const [search, setSearch] = useState('')
  const [logScale, setLogScale] = useState(false)
  const [selected, setSelected] = useState<string[]>(() =>
    defaultSelection(points, meta, defaultSelectionCount)
  )

  const filteredMeta = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return meta
    return meta.filter((m) => m.label.toLowerCase().includes(q))
  }, [meta, search])

  const seriesColor = useMemo(() => {
    const colors = new Map<string, string>()
    meta.forEach((m, i) => colors.set(m.key, CATEGORICAL_PALETTE[i % CATEGORICAL_PALETTE.length]))
    return colors
  }, [meta])

  const atSelectionCap = selected.length >= CATEGORICAL_PALETTE.length

  function toggleSeries(key: string) {
    setSelected((prev) =>
      prev.includes(key)
        ? prev.filter((k) => k !== key)
        : prev.length >= CATEGORICAL_PALETTE.length
          ? prev
          : [...prev, key]
    )
  }

  const chartData = useMemo(
    () => pivotSeries(points.filter((p) => selected.includes(p.seriesKey))),
    [points, selected]
  )
  const domain = useMemo(
    () => logSafeDomain(points.filter((p) => selected.includes(p.seriesKey))),
    [points, selected]
  )

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
      <div className="lg:col-span-1">
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={searchPlaceholder}
          className="mb-2"
        />
        <Checkbox
          id="multi-series-log-scale"
          label="Log scale"
          checked={logScale}
          onChange={(e) => setLogScale(e.target.checked)}
          className="mb-2"
        />
        <div className="max-h-64 space-y-1 overflow-y-auto rounded-lg border border-border p-2">
          {filteredMeta.length === 0 ? (
            <p className="py-2 text-center text-xs text-muted">No matching series.</p>
          ) : (
            filteredMeta.map((m) => {
              const isSelected = selected.includes(m.key)
              return (
                <Checkbox
                  key={m.key}
                  id={`multi-series-${m.key}`}
                  label={m.label}
                  checked={isSelected}
                  disabled={!isSelected && atSelectionCap}
                  onChange={() => toggleSeries(m.key)}
                  className="block"
                />
              )
            })
          )}
        </div>
      </div>

      <div className="lg:col-span-2">
        {chartData.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">Select at least one series to plot.</p>
        ) : (
          <div className="h-72 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData} margin={{ left: 8, right: 16 }}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis
                  dataKey="day"
                  tick={{ fontSize: 11 }}
                  minTickGap={32}
                  tickFormatter={xAxisTickFormatter}
                />
                <YAxis
                  scale={logScale ? 'log' : 'linear'}
                  domain={logScale ? domain : ['auto', 'auto']}
                  tick={{ fontSize: 11 }}
                  tickFormatter={logScale ? logAxisTickFormatter : valueFormatter}
                  allowDataOverflow={logScale}
                />
                <Tooltip
                  contentStyle={chartTooltipStyle}
                  labelStyle={{ color: 'var(--color-fg)' }}
                  itemStyle={{ color: 'var(--color-fg)' }}
                  labelFormatter={tooltipLabelFormatter}
                  formatter={(value, name) => [
                    valueFormatter(Number(value)),
                    meta.find((m) => m.key === name)?.label ?? String(name)
                  ]}
                />
                {selected.map((key) => (
                  <Line
                    key={key}
                    type="monotone"
                    dataKey={key}
                    name={key}
                    stroke={seriesColor.get(key)}
                    strokeWidth={2}
                    dot={false}
                    isAnimationActive={false}
                    connectNulls
                  />
                ))}
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
        <p className="mt-2 text-xs text-muted">{valueLabel}</p>
      </div>
    </div>
  )
}

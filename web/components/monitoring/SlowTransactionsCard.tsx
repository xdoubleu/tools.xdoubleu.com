'use client'

import {
  BarChart,
  Bar,
  Cell,
  LabelList,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type {
  GetSlowTransactionsResponse,
  GetAlertStatesResponse,
  SlowTransaction
} from '@/lib/gen/observability/v1/observability_pb'
import {
  formatCount,
  formatDuration,
  slowTransactionThresholds,
  isSlowTransaction,
  CATEGORICAL_PALETTE,
  chartTooltipStyle
} from '@/lib/observability'

function formatPctChange(pctChange: number): string {
  return `+${Math.round(pctChange * 100)}%`
}

// regressionDangerThreshold flags a span as a real regression (danger
// tone) rather than just "trending" (warn tone) once its p95 has grown by
// more than half — no backend threshold config exists for this (issue
// #1261).
export const regressionDangerThreshold = 0.5

function pctChangeVariant(pctChange: number): 'danger' | 'warn' {
  return pctChange > regressionDangerThreshold ? 'danger' : 'warn'
}

// SpanBar is one plotted row — a (project, span) pair's current p95
// duration, request count, and whether it's over its class threshold.
export interface SpanBar {
  key: string
  label: string
  p95: number
  requests: number
  slow: boolean
}

// toSpanBars pivots the raw SlowTransaction rows into the flat shape the
// bar chart plots, sorted slowest-first (the API already returns `current`
// sorted this way, but re-sorting here keeps the chart correct even if a
// caller passes an already-filtered subset in a different order).
export function toSpanBars(
  current: SlowTransaction[],
  thresholds: Record<string, number>
): SpanBar[] {
  return current
    .map((t) => ({
      key: `${t.project}-${t.transaction}`,
      label: `${t.project} · ${t.transaction}`,
      p95: t.p95DurationMs,
      requests: Number(t.requestCount),
      slow: isSlowTransaction(t.transaction, t.p95DurationMs, thresholds)
    }))
    .sort((a, b) => b.p95 - a.p95)
}

function SpanTooltip({ active, payload }: { active?: boolean; payload?: { payload: SpanBar }[] }) {
  const bar = payload?.[0]?.payload
  if (!active || !bar) return null
  return (
    <div style={chartTooltipStyle} className="rounded-xl border px-3 py-2 text-xs">
      <p className="font-medium text-fg">{bar.label}</p>
      <p className="mt-1 text-fg">
        {formatDuration(bar.p95)} p95 · {formatCount(bar.requests)} requests
      </p>
    </div>
  )
}

export default function SlowTransactionsCard({
  data,
  alertStates,
  filtered = false
}: {
  data?: GetSlowTransactionsResponse
  alertStates?: GetAlertStatesResponse
  // filtered restricts `current` to rows currently breaching their class's
  // threshold, and hides the "Getting slower" trending section entirely —
  // used on the Issues page (issue #1308), which shows only what currently
  // needs attention, unlike the unfiltered exhaustive view on
  // /monitoring/observability.
  filtered?: boolean
}) {
  const thresholds = slowTransactionThresholds(alertStates)
  const allCurrent = data?.current ?? []
  const current = filtered
    ? allCurrent.filter((t) => isSlowTransaction(t.transaction, t.p95DurationMs, thresholds))
    : allCurrent
  const trending = filtered ? [] : (data?.trending ?? [])

  const bars = toSpanBars(current, thresholds)
  const hasSlowBar = bars.some((b) => b.slow)
  const chartHeight = Math.max(160, bars.length * 32)

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Slow spans</CardTitle>
          <Badge variant="secondary">Sentry</Badge>
        </div>
        <CardDescription>
          {filtered
            ? 'Spans currently over their class threshold.'
            : 'Slowest API endpoints/pages right now, plus ones getting slower over time.'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">Sentry is not configured.</p>
        ) : bars.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">
            {filtered ? 'No spans currently over threshold.' : 'No spans recorded yet.'}
          </p>
        ) : (
          <>
            <div style={{ height: chartHeight }} className="w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={bars} layout="vertical" margin={{ left: 8, right: 56 }}>
                  <CartesianGrid strokeDasharray="3 3" horizontal={false} />
                  <XAxis type="number" tickFormatter={formatDuration} tick={{ fontSize: 11 }} />
                  <YAxis
                    type="category"
                    dataKey="label"
                    width={220}
                    tick={{ fontSize: 11 }}
                    interval={0}
                  />
                  <Tooltip
                    content={<SpanTooltip />}
                    cursor={{ fill: 'rgb(var(--hover-rgb) / 0.5)' }}
                  />
                  <Bar dataKey="p95" radius={[0, 4, 4, 0]} isAnimationActive={false}>
                    {bars.map((b) => (
                      <Cell
                        key={b.key}
                        fill={b.slow ? 'var(--color-danger)' : CATEGORICAL_PALETTE[0]}
                      />
                    ))}
                    <LabelList
                      dataKey="p95"
                      position="right"
                      formatter={(v: unknown) => formatDuration(Number(v))}
                      style={{ fill: 'var(--color-muted)', fontSize: 11 }}
                    />
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
            {!filtered && hasSlowBar && (
              <p className="mt-2 flex items-center gap-1.5 text-xs text-muted">
                <span
                  aria-hidden
                  className="inline-block h-2 w-2 rounded-full"
                  style={{ backgroundColor: 'var(--color-danger)' }}
                />
                Over its class threshold
              </p>
            )}
          </>
        )}

        {trending.length > 0 && (
          <div className="mt-5">
            <h4 className="mb-2 text-sm font-semibold text-subtle">Getting slower</h4>
            <ul className="space-y-2">
              {trending.map((t) => (
                <li
                  key={`${t.project}-${t.transaction}`}
                  className="rounded-lg border border-border bg-surface p-3 text-sm"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="break-words font-mono text-xs text-fg">{t.transaction}</span>
                    <div className="flex shrink-0 items-center gap-1">
                      <Badge variant="secondary">{t.project}</Badge>
                      <Badge variant={pctChangeVariant(t.pctChange)}>
                        {formatPctChange(t.pctChange)}
                      </Badge>
                    </div>
                  </div>
                  <p className="mt-1 text-xs text-muted">
                    {formatDuration(t.priorAvgP95Ms)} → {formatDuration(t.recentAvgP95Ms)}
                  </p>
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

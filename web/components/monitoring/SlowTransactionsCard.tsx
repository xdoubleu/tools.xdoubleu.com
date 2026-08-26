'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetSlowTransactionsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatCount, formatDuration } from '@/lib/observability'

function formatPctChange(pctChange: number): string {
  return `+${Math.round(pctChange * 100)}%`
}

export default function SlowTransactionsCard({ data }: { data?: GetSlowTransactionsResponse }) {
  const current = data?.current ?? []
  const trending = data?.trending ?? []

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Slow transactions</CardTitle>
          <Badge variant="secondary">Sentry</Badge>
        </div>
        <CardDescription>
          Slowest API endpoints/pages right now, plus ones getting slower over time.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">Sentry is not configured.</p>
        ) : current.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No transactions recorded yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border">
                <tr>
                  <th className="py-2 pr-3 font-semibold text-subtle">Transaction</th>
                  <th className="py-2 pr-3 font-semibold text-subtle">Project</th>
                  <th className="py-2 pr-3 text-right font-semibold text-subtle">p95</th>
                  <th className="py-2 text-right font-semibold text-subtle">Requests</th>
                </tr>
              </thead>
              <tbody>
                {current.map((t) => (
                  <tr
                    key={`${t.project}-${t.transaction}`}
                    className="border-b border-border last:border-0"
                  >
                    <td className="py-2 pr-3 font-mono text-xs text-fg">{t.transaction}</td>
                    <td className="py-2 pr-3">
                      <Badge variant="secondary">{t.project}</Badge>
                    </td>
                    <td className="py-2 pr-3 text-right text-fg">
                      {formatDuration(t.p95DurationMs)}
                    </td>
                    <td className="py-2 text-right text-fg">{formatCount(t.requestCount)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
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
                      <Badge variant="danger">{formatPctChange(t.pctChange)}</Badge>
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

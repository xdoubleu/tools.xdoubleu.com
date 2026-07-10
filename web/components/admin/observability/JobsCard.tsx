'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetJobStatsResponse } from '@/lib/gen/admin/v1/admin_pb'
import { formatCount, formatDuration, successRate } from '@/lib/observability'
import { formatDateTime } from '@/lib/dates'

function formatWhen(rfc3339: string): string {
  return formatDateTime(rfc3339) || '—'
}

export default function JobsCard({ data }: { data?: GetJobStatsResponse }) {
  const stats = data?.stats ?? []
  const failures = (data?.recentRuns ?? []).filter((r) => !r.success).slice(0, 8)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Background jobs</CardTitle>
        <CardDescription>Throughput and failures over the selected window.</CardDescription>
      </CardHeader>
      <CardContent>
        {stats.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No job runs recorded.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border">
                <tr>
                  <th className="py-2 pr-3 font-semibold text-subtle">Job</th>
                  <th className="py-2 pr-3 text-right font-semibold text-subtle">Runs</th>
                  <th className="py-2 pr-3 text-right font-semibold text-subtle">Success</th>
                  <th className="py-2 pr-3 text-right font-semibold text-subtle">Avg</th>
                  <th className="py-2 text-right font-semibold text-subtle">Last run</th>
                </tr>
              </thead>
              <tbody>
                {stats.map((s) => {
                  const rate = successRate(s.totalRuns, s.failedRuns)
                  return (
                    <tr key={s.jobId} className="border-b border-border last:border-0">
                      <td className="py-2 pr-3 font-mono text-xs text-fg">{s.jobId}</td>
                      <td className="py-2 pr-3 text-right text-fg">{formatCount(s.totalRuns)}</td>
                      <td className="py-2 pr-3 text-right">
                        <Badge variant={rate === 100 ? 'success' : rate >= 90 ? 'warn' : 'danger'}>
                          {rate}%
                        </Badge>
                      </td>
                      <td className="py-2 pr-3 text-right text-fg">
                        {formatDuration(s.avgDurationMs)}
                      </td>
                      <td className="py-2 text-right text-muted">{formatWhen(s.lastRunAt)}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}

        {failures.length > 0 && (
          <div className="mt-5">
            <h4 className="mb-2 text-sm font-semibold text-subtle">Recent failures</h4>
            <ul className="space-y-2">
              {failures.map((r, i) => (
                <li
                  key={`${r.jobId}-${r.startedAt}-${i}`}
                  className="rounded-lg border border-border bg-surface p-3 text-sm"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-fg">{r.jobId}</span>
                    <span className="text-xs text-muted">{formatWhen(r.startedAt)}</span>
                  </div>
                  {r.error && <p className="mt-1 break-words text-xs text-danger">{r.error}</p>}
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

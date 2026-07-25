'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetJobStatsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatDateTime } from '@/lib/dates'

function formatWhen(rfc3339: string): string {
  return formatDateTime(rfc3339) || '—'
}

const STATUS_TONE: Record<string, 'success' | 'warn' | 'danger'> = {
  ok: 'success',
  active: 'warn',
  error: 'danger',
  missed_checkin: 'danger',
  timeout: 'danger',
  disabled: 'danger'
}

export default function JobsCard({ data }: { data?: GetJobStatsResponse }) {
  const monitors = data?.monitors ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>Background jobs</CardTitle>
        <CardDescription>Cron monitor status via Sentry.</CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">Sentry is not configured.</p>
        ) : monitors.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No job monitors recorded.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border">
                <tr>
                  <th className="py-2 pr-3 font-semibold text-subtle">Job</th>
                  <th className="py-2 pr-3 text-right font-semibold text-subtle">Status</th>
                  <th className="py-2 pr-3 text-right font-semibold text-subtle">Last check-in</th>
                  <th className="py-2 text-right font-semibold text-subtle">Next check-in</th>
                </tr>
              </thead>
              <tbody>
                {monitors.map((m) => (
                  <tr key={m.slug} className="border-b border-border last:border-0">
                    <td className="py-2 pr-3 font-mono text-xs text-fg">{m.slug}</td>
                    <td className="py-2 pr-3 text-right">
                      <Badge variant={STATUS_TONE[m.status] ?? 'secondary'}>{m.status}</Badge>
                    </td>
                    <td className="py-2 pr-3 text-right text-muted">{formatWhen(m.lastCheckIn)}</td>
                    <td className="py-2 text-right text-muted">{formatWhen(m.nextCheckIn)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

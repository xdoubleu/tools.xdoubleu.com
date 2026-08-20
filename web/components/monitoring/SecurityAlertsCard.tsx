'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge, type BadgeProps } from '@/components/ui/badge'
import type { GetSecurityAlertsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatCount } from '@/lib/observability'
import { formatDate } from '@/lib/dates'

const SEVERITY_VARIANT: Record<string, BadgeProps['variant']> = {
  critical: 'danger',
  high: 'danger',
  medium: 'warn',
  low: 'secondary'
}

export default function SecurityAlertsCard({ data }: { data?: GetSecurityAlertsResponse }) {
  const alerts = data?.alerts ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>Security alerts</CardTitle>
        <CardDescription>
          {data ? `${formatCount(data.alertCount)} open Dependabot alert(s).` : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">GitHub is not configured.</p>
        ) : alerts.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No open security alerts.</p>
        ) : (
          <ul className="space-y-2">
            {alerts.map((alert) => (
              <li
                key={alert.number}
                className="rounded-lg border border-border bg-surface p-3 text-sm"
              >
                <div className="flex items-start justify-between gap-2">
                  <a
                    href={alert.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="break-words font-medium text-fg hover:text-accent"
                  >
                    <span className="mr-1 font-mono text-xs text-muted">#{alert.number}</span>
                    {alert.packageName}
                  </a>
                  <span className="shrink-0 text-xs text-muted">
                    {alert.ecosystem} &middot; {formatDate(alert.createdAt)}
                  </span>
                </div>
                <p className="mt-1 text-xs text-muted">{alert.summary}</p>
                <div className="mt-2">
                  <Badge variant={SEVERITY_VARIANT[alert.severity] ?? 'default'}>
                    {alert.severity}
                  </Badge>
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}

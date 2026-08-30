'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type {
  AlertState,
  GetAlertStatesResponse
} from '@/lib/gen/observability/v1/observability_pb'
import { formatBytes, formatDuration } from '@/lib/observability'
import { formatDateTime } from '@/lib/dates'

// AlertState carries only a rule_key and bare doubles — the rule's own
// label and unit live in the Go rule table (buildAlertRules in
// api/internal/observability/jobs/threshold_alert.go) and aren't on the
// wire, so they're mirrored here. Keep both sides in sync when a rule is
// added or its unit changes.
const RULE_LABELS: Record<string, string> = {
  host_cpu_high: 'Host CPU usage',
  host_memory_high: 'Host memory usage',
  host_disk_high: 'Host disk usage',
  r2_usage_high: 'R2 storage usage',
  ci_duration_high: 'CI workflow duration (p95)',
  slow_transaction_http_high: 'Slow HTTP handlers (p95)',
  slow_transaction_job_high: 'Slow background jobs (p95)',
  slow_transaction_frontend_high: 'Slow frontend transactions (p95)'
}

const RULE_FORMATTERS: Record<string, (value: number) => string> = {
  host_cpu_high: formatPercent,
  host_memory_high: formatPercent,
  host_disk_high: formatPercent,
  r2_usage_high: (value) => formatBytes(value),
  ci_duration_high: (value) => formatDuration(value),
  slow_transaction_http_high: (value) => formatDuration(value),
  slow_transaction_job_high: (value) => formatDuration(value),
  slow_transaction_frontend_high: (value) => formatDuration(value)
}

function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

function ruleLabel(ruleKey: string): string {
  return RULE_LABELS[ruleKey] ?? ruleKey
}

// formatValue falls back to a plain number for an unknown rule key so a
// rule added server-side renders readably instead of throwing.
function formatValue(ruleKey: string, value: number): string {
  const formatter = RULE_FORMATTERS[ruleKey]
  return formatter ? formatter(value) : String(value)
}

// breachingFirst sorts tripped rules to the top so a breach is visible
// without scanning the whole list.
function breachingFirst(states: AlertState[]): AlertState[] {
  return [...states].sort((a, b) => Number(b.breaching) - Number(a.breaching))
}

export default function AlertStatesCard({ data }: { data?: GetAlertStatesResponse }) {
  const states = data ? breachingFirst(data.states) : []
  const breachingCount = states.filter((s) => s.breaching).length

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Threshold alerts</CardTitle>
          {breachingCount > 0 && <Badge variant="danger">{breachingCount} breaching</Badge>}
        </div>
        <CardDescription>
          {data ? 'Host, storage, and CI thresholds evaluated every 5 minutes.' : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && states.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No threshold alert rules.</p>
        ) : (
          <ul className="space-y-2">
            {states.map((state) => (
              <li
                key={state.ruleKey}
                className="rounded-lg border border-border bg-surface p-3 text-sm"
              >
                <div className="flex items-start justify-between gap-2">
                  <span className="break-words font-medium text-fg">
                    {ruleLabel(state.ruleKey)}
                  </span>
                  <Badge variant={state.breaching ? 'danger' : 'success'}>
                    {state.breaching ? 'Breaching' : 'OK'}
                  </Badge>
                </div>
                <p className="mt-1 text-xs text-muted">
                  {formatValue(state.ruleKey, state.currentValue)} of{' '}
                  {formatValue(state.ruleKey, state.threshold)} threshold
                </p>
                {state.breaching && state.since && (
                  <p className="mt-1 text-xs text-muted">Since {formatDateTime(state.since)}</p>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}

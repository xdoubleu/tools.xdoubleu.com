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

// approachingRatio is how close a rule needs to get to its threshold, as a
// fraction, before the meter switches from "good" to "approaching" — the
// server itself only reports a boolean breaching flag, so this is purely a
// client-side visual cue that a rule is trending toward a breach.
const approachingRatio = 0.8

// meterRatio clamps to [0, 1] so an already-breaching rule (current value
// past its threshold) still fills the bar fully rather than overflowing it.
function meterRatio(state: AlertState): number {
  if (state.threshold <= 0) return 0
  return Math.min(state.currentValue / state.threshold, 1)
}

type MeterTone = 'success' | 'warn' | 'danger'

function meterTone(state: AlertState, ratio: number): MeterTone {
  if (state.breaching) return 'danger'
  if (ratio >= approachingRatio) return 'warn'
  return 'success'
}

const METER_FILL_CLASSES: Record<MeterTone, string> = {
  success: 'bg-success',
  warn: 'bg-warn',
  danger: 'bg-danger'
}

// AlertMeter gives each rule a visual sense of how close it is to its
// threshold — the plain "X of Y threshold" text alone made every rule look
// the same at a glance regardless of whether it was nowhere near tripping
// or one step away (issue #1350).
function AlertMeter({ state }: { state: AlertState }) {
  const ratio = meterRatio(state)
  const tone = meterTone(state, ratio)

  return (
    <div
      className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-border"
      role="progressbar"
      aria-valuenow={Math.round(ratio * 100)}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={`${ruleLabel(state.ruleKey)} threshold usage`}
    >
      <div
        className={`h-full rounded-full ${METER_FILL_CLASSES[tone]}`}
        style={{ width: `${ratio * 100}%` }}
      />
    </div>
  )
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
                <AlertMeter state={state} />
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

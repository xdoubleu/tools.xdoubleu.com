'use client'

import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis
} from 'recharts'
import { axisTick, tooltipProps } from '@/lib/chartTheme'
import type { HitRate, RankedName } from '@/lib/gen/taste/v1/taste_pb'
import { useTasteStats } from '@/hooks/useTaste'
import { DOMAINS, domainLabel, hitRateLabel, monthRows, trackLabel } from '@/lib/taste/format'
import { formatDate } from '@/lib/dates'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatTile } from '@/components/ui/stat'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function RankedList({
  title,
  names,
  empty
}: {
  title: string
  names: RankedName[]
  empty: string
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {names.length === 0 ? (
          <p className="text-sm text-muted">{empty}</p>
        ) : (
          <ol className="space-y-2 text-sm">
            {names.map((n) => (
              <li key={n.name} className="flex items-start justify-between gap-3">
                <span className="min-w-0">
                  <span className="break-words">{n.name}</span>
                  <span className="mt-0.5 flex flex-wrap gap-1">
                    {n.domains.map((d) => (
                      <Badge key={d} variant="secondary">
                        {domainLabel(d)}
                      </Badge>
                    ))}
                  </span>
                </span>
                <span className="text-muted">{n.positive}</span>
              </li>
            ))}
          </ol>
        )}
      </CardContent>
    </Card>
  )
}

function HitRates({ rates }: { rates: HitRate[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recommendation hit rate</CardTitle>
      </CardHeader>
      <CardContent>
        <ul className="space-y-2 text-sm" aria-label="Hit rate per domain and track">
          {rates.map((r) => (
            <li key={`${r.domain}:${r.track}`} className="flex items-start justify-between gap-3">
              <span className="min-w-0 break-words">
                {domainLabel(r.domain)} · {trackLabel(r.track)}
              </span>
              <span className="shrink-0 text-muted">{hitRateLabel(r.hits, r.misses)}</span>
            </li>
          ))}
        </ul>
        <p className="mt-2 text-xs text-muted">Hits out of the picks you marked hit or miss.</p>
      </CardContent>
    </Card>
  )
}

/** Per-domain totals, consumption per month and what you like most. */
export default function TasteStats() {
  const { data, error, isLoading } = useTasteStats()

  if (isLoading && !data) return <LoadingState label="stats" />
  if (error && !data) return <ErrorState what="stats" />
  if (!data) return null
  if (data.domains.every((d) => d.consumed === 0)) {
    return (
      <EmptyState>
        Nothing consumed yet. Finish a book, read an article, play a game, watch something or
        favourite a podcast to see it here.
      </EmptyState>
    )
  }

  const rows = monthRows(data.months)

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {data.domains.map((d) => (
          <StatTile
            key={d.domain}
            label={domainLabel(d.domain)}
            value={d.consumed}
            hint={
              <span suppressHydrationWarning>
                {d.positive} liked · {d.negative} disliked
                {d.lastAt && <> · last {formatDate(d.lastAt)}</>}
              </span>
            }
          />
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Consumed per month</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-64 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={rows}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="month" tick={axisTick} />
                <YAxis allowDecimals={false} tick={axisTick} width={32} />
                <Tooltip {...tooltipProps} />
                <Legend />
                {DOMAINS.map((d) => (
                  <Bar key={d.id} dataKey={d.label} stackId="events" fill={d.color} />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </div>
          <p className="mt-2 text-xs text-muted">Items without a date aren&apos;t charted.</p>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <RankedList title="Top tags" names={data.topTags} empty="No liked tags yet." />
        <RankedList title="Top creators" names={data.topCreators} empty="No liked creators yet." />
      </div>

      {data.hitRates.length > 0 && <HitRates rates={data.hitRates} />}
    </div>
  )
}

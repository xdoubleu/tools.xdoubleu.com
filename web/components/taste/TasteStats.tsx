import { Bar, BarChart, CartesianGrid, Legend, Tooltip, XAxis, YAxis } from 'recharts'
import { axisTick, countAxisProps, gridProps, tooltipProps } from '@/lib/chartTheme'
import type { HitRate, RankedName } from '@/lib/gen/taste/v1/taste_pb'
import { useTasteStats } from '@/hooks/useTaste'
import { DOMAINS, domainLabel, hitRateLabel, monthRows, trackLabel } from '@/lib/taste/format'
import { formatDate } from '@/lib/dates'
import { Badge } from '@/components/ui/badge'
import { ChartCard, RankedList, StatTile, StatTileGrid } from '@/components/ui/stat'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function rankedItems(names: RankedName[]) {
  return names.map((n) => ({
    key: n.name,
    label: n.name,
    value: n.positive,
    sub: n.domains.map((d) => (
      <Badge key={d} variant="secondary">
        {domainLabel(d)}
      </Badge>
    ))
  }))
}

function HitRates({ rates }: { rates: HitRate[] }) {
  return (
    <RankedList
      title="Recommendation hit rate"
      empty="No picks marked hit or miss yet."
      note="Hits out of the picks you marked hit or miss."
      items={rates.map((r) => ({
        key: `${r.domain}:${r.track}`,
        label: `${domainLabel(r.domain)} · ${trackLabel(r.track)}`,
        value: hitRateLabel(r.hits, r.misses)
      }))}
    />
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
      <StatTileGrid columns={3}>
        {data.domains.map((d) => (
          <StatTile
            key={d.domain}
            label={domainLabel(d.domain)}
            value={d.consumed}
            hint={
              <span>
                {d.positive} liked · {d.negative} disliked
                {d.lastAt && <> · last {formatDate(d.lastAt)}</>}
              </span>
            }
          />
        ))}
      </StatTileGrid>

      <ChartCard title="Consumed per month" note="Items without a date aren’t charted.">
        <BarChart data={rows}>
          <CartesianGrid {...gridProps} />
          <XAxis dataKey="month" tick={axisTick} />
          <YAxis {...countAxisProps} />
          <Tooltip {...tooltipProps} />
          <Legend />
          {DOMAINS.map((d) => (
            <Bar key={d.id} dataKey={d.label} stackId="events" fill={d.color} />
          ))}
        </BarChart>
      </ChartCard>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <RankedList title="Top tags" items={rankedItems(data.topTags)} empty="No liked tags yet." />
        <RankedList
          title="Top creators"
          items={rankedItems(data.topCreators)}
          empty="No liked creators yet."
        />
      </div>

      {data.hitRates.length > 0 && <HitRates rates={data.hitRates} />}
    </div>
  )
}

import { Link } from '@/components/ui/link'
import { type ReactElement, type ReactNode } from 'react'
import { ResponsiveContainer } from 'recharts'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  interactiveCardClass
} from '@/components/ui/card'
import { EmptyState } from '@/components/ui/states'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'

type StatTone = 'default' | 'success' | 'warn' | 'danger'

const toneClasses: Record<StatTone, string> = {
  default: 'text-fg',
  success: 'text-success',
  warn: 'text-warn',
  danger: 'text-danger'
}

interface StatTileProps {
  label: string
  value: ReactNode
  /** Colours the value only — the label stays muted so tiles scan as one row. */
  tone?: StatTone
  /** Optional muted line under the value (a delta, a unit, a timestamp). */
  hint?: ReactNode
  /** Renders the tile as a navigable card with a pending-navigation spinner. */
  href?: string
  className?: string
}

/** One labelled number in a stats row; structured content belongs in a `SectionCard`. */
function StatTile({ label, value, tone = 'default', hint, href, className }: StatTileProps) {
  const body = (
    <>
      <p className="text-xs font-medium uppercase tracking-wide text-muted">{label}</p>
      <p className={cn('mt-1 text-2xl font-semibold', toneClasses[tone])}>{value}</p>
      {hint !== undefined && <p className="mt-0.5 text-xs text-muted">{hint}</p>}
    </>
  )

  if (href !== undefined) {
    return (
      <Link href={href} className={cn(interactiveCardClass, 'relative block p-4', className)}>
        <CardLinkStatus />
        {body}
      </Link>
    )
  }

  return <Card className={className}>{body}</Card>
}

/** Responsive grid for a row of `StatTile`s: two up on mobile, `columns` from `sm`. */
function StatTileGrid({
  className,
  children,
  columns = 4
}: {
  className?: string
  children: ReactNode
  columns?: 3 | 4
}) {
  return (
    <div
      className={cn(
        'grid grid-cols-2 gap-3',
        columns === 3 ? 'sm:grid-cols-3' : 'sm:grid-cols-4',
        className
      )}
    >
      {children}
    </div>
  )
}

interface ChartCardProps {
  title: string
  /** One recharts chart; sized to the card. Style it from `lib/chartTheme`. */
  children: ReactElement
  /** Shown instead of the chart when there is nothing to plot. */
  empty?: string
  /** Muted footnote under the chart, e.g. what is left out. */
  note?: ReactNode
}

/** A titled card holding one chart: the second step of the stats layout (tiles, charts, lists). */
function ChartCard({ title, children, empty, note }: ChartCardProps) {
  return (
    <Card padding="none">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {empty ? (
          <EmptyState placement="inline">{empty}</EmptyState>
        ) : (
          <div className="h-64 w-full">
            <ResponsiveContainer width="100%" height="100%">
              {children}
            </ResponsiveContainer>
          </div>
        )}
        {note && !empty && <p className="mt-2 text-xs text-muted">{note}</p>}
      </CardContent>
    </Card>
  )
}

interface RankedItem {
  key: string
  label: ReactNode
  value: ReactNode
  /** Muted line under the label, e.g. badges. */
  sub?: ReactNode
}

interface RankedListProps {
  title: string
  items: RankedItem[]
  /** Shown when `items` is empty. */
  empty: string
  note?: ReactNode
}

/** A titled card of label/value rows: the last step of the stats layout. */
function RankedList({ title, items, empty, note }: RankedListProps) {
  return (
    <Card padding="none">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <EmptyState placement="inline">{empty}</EmptyState>
        ) : (
          <ol aria-label={title} className="space-y-2 text-sm">
            {items.map((item) => (
              <li key={item.key} className="flex items-start justify-between gap-3">
                <span className="min-w-0 break-words">
                  {item.label}
                  {item.sub && <span className="mt-0.5 flex flex-wrap gap-1">{item.sub}</span>}
                </span>
                <span className="shrink-0 text-muted">{item.value}</span>
              </li>
            ))}
          </ol>
        )}
        {note && items.length > 0 && <p className="mt-2 text-xs text-muted">{note}</p>}
      </CardContent>
    </Card>
  )
}

export { StatTile, StatTileGrid, ChartCard, RankedList }
export type { StatTileProps, StatTone, ChartCardProps, RankedListProps, RankedItem }

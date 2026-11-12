import { type ReactNode } from 'react'
import { Link } from '@/components/ui/link'
import { Button } from '@/components/ui/button'
import { ProgressBar } from '@/components/ui/progress-bar'
import { SectionCard } from '@/components/ui/section-card'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState, ErrorState } from '@/components/ui/states'
import { useToday } from '@/hooks/useToday'
import type { GetTodayResponse } from '@/lib/gen/dashboard/v1/today_pb'

const SLOT_LABELS: Record<string, string> = {
  breakfast: 'Breakfast',
  noon: 'Lunch',
  evening: 'Dinner'
}

function Row({ href, title, detail }: { href: string; title: string; detail?: ReactNode }) {
  return (
    <li>
      <Button asChild variant="link" className="w-full justify-between gap-3">
        <Link href={href}>
          <span className="truncate">{title}</span>
          {detail !== undefined && <span className="shrink-0 text-xs text-muted">{detail}</span>}
        </Link>
      </Button>
    </li>
  )
}

// Stryker disable StringLiteral: React keys are not observable
function Blocks({ today }: { today: GetTodayResponse }) {
  const { reading, feeds, meals, shopping, watching, learning, factory } = today
  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
      {factory && factory.tasks.length > 0 && (
        <SectionCard title="Needs you">
          <ul>
            {factory.tasks.map((t) => (
              <Row
                key={t.ref}
                href={`/factory?task=${t.ref}`}
                title={`${t.ref} ${t.title}`}
                detail={t.status === 'needs_you' ? 'Needs you' : 'Testing'}
              />
            ))}
          </ul>
        </SectionCard>
      )}
      {meals && meals.meals.length > 0 && (
        <SectionCard title="Meals">
          <ul>
            {meals.meals.map((m) => (
              <Row
                key={`${m.date}-${m.slot}-${m.name}`}
                href="/mealplans"
                title={m.name}
                detail={SLOT_LABELS[m.slot] ?? m.slot}
              />
            ))}
          </ul>
        </SectionCard>
      )}
      {reading && reading.books.length > 0 && (
        <SectionCard title="Reading">
          <ul className="space-y-3">
            {reading.books.map((b) => (
              <li key={b.id} className="space-y-1">
                <Button asChild variant="link" className="w-full justify-start">
                  <Link href={`/books/${b.id}`}>
                    <span className="truncate">{b.title}</span>
                  </Link>
                </Button>
                <ProgressBar
                  value={b.progressPercent}
                  aria-label={`${b.title} progress`}
                  valueText={`${b.progressPercent}%`}
                />
              </li>
            ))}
          </ul>
        </SectionCard>
      )}
      {feeds && feeds.unreadCount > 0 && (
        <SectionCard
          title="Feeds"
          action={
            <Button asChild variant="link">
              <Link href="/feeds">{feeds.unreadCount} unread</Link>
            </Button>
          }
        >
          <ul>
            {feeds.items.map((i) => (
              <Row key={i.id} href={i.url} title={i.title} detail={i.feedTitle} />
            ))}
          </ul>
        </SectionCard>
      )}
      {watching && watching.titles.length > 0 && (
        <SectionCard title="Watching">
          <ul>
            {watching.titles.map((t) => (
              <Row
                key={t.id}
                href={`/movies/${t.id}`}
                title={t.title}
                detail={t.hasNewSeason ? 'New season' : undefined}
              />
            ))}
          </ul>
        </SectionCard>
      )}
      {learning && learning.paths.length > 0 && (
        <SectionCard title="Learning">
          <ul>
            {learning.paths.map((p) => (
              <Row key={p.id} href={`/learningpaths/${p.id}`} title={p.title} />
            ))}
          </ul>
        </SectionCard>
      )}
      {shopping && shopping.itemCount > 0 && (
        <SectionCard title="Shopping list">
          <Button asChild variant="link">
            <Link href="/shoppinglist">
              {shopping.itemCount} {shopping.itemCount === 1 ? 'item' : 'items'} on your list
            </Link>
          </Button>
        </SectionCard>
      )}
    </div>
  )
}

// Stryker restore StringLiteral
function hasContent(t: GetTodayResponse): boolean {
  return (
    (t.factory?.tasks.length ?? 0) > 0 ||
    (t.meals?.meals.length ?? 0) > 0 ||
    (t.reading?.books.length ?? 0) > 0 ||
    (t.feeds?.unreadCount ?? 0) > 0 ||
    (t.watching?.titles.length ?? 0) > 0 ||
    (t.learning?.paths.length ?? 0) > 0 ||
    (t.shopping?.itemCount ?? 0) > 0
  )
}

/** What needs attention today, across the apps the user can use. */
export default function TodayView() {
  const { data, error } = useToday()

  if (error && !data) return <ErrorState what="your day" placement="page" />

  if (!data) {
    return (
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2" aria-busy="true">
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-32" />
        ))}
      </div>
    )
  }

  if (!hasContent(data)) {
    return (
      <EmptyState
        action={
          <Button asChild>
            <Link href="/apps">Browse apps</Link>
          </Button>
        }
      >
        Nothing needs you today. Your apps are one tap away.
      </EmptyState>
    )
  }

  return <Blocks today={data} />
}

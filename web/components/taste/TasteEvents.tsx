'use client'

import { useMemo, useState } from 'react'
import type { Event } from '@/lib/gen/taste/v1/taste_pb'
import { useFetchTasteEventsPage, useTasteEvents } from '@/hooks/useTaste'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import { DOMAINS, domainLabel, signalBadge } from '@/lib/taste/format'
import { formatDate } from '@/lib/dates'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { Select } from '@/components/ui/select'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function EventRow({ event }: { event: Event }) {
  const badge = signalBadge(event.signal)
  const meta = [domainLabel(event.domain), event.creator, formatDate(event.at)].filter(Boolean)
  return (
    <Card variant="inset" className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <p className="break-words font-medium">{event.title}</p>
        <p className="break-words text-xs text-muted" suppressHydrationWarning>
          {meta.join(' · ')}
        </p>
        {event.reason && <p className="text-sm text-subtle">{event.reason}</p>}
      </div>
      <Badge variant={badge.variant}>{badge.label}</Badge>
    </Card>
  )
}

function EventList({ domain }: { domain: string }) {
  const { data, error, isLoading } = useTasteEvents(domain)
  const fetchPage = useFetchTasteEventsPage(domain)
  const initialPage = useMemo(
    // Stryker disable next-line BooleanLiteral: without data there are no events to load more of.
    () => ({ items: data?.events ?? [], hasMore: data?.hasMore ?? false }),
    [data]
  )
  const {
    items: events,
    hasMore,
    loading: loadingMore,
    loadMore
  } = usePaginatedList(
    initialPage,
    fetchPage,
    (a, b) => a.domain === b.domain && a.itemId === b.itemId
  )

  if (error && !data) return <ErrorState what="log" />
  if (isLoading && !data) return <LoadingState label="log" />
  if (events.length === 0) return <EmptyState>Nothing here yet.</EmptyState>

  return (
    <>
      <ul className="space-y-2" aria-label="Consumption log">
        {events.map((e) => (
          <li key={`${e.domain}:${e.itemId}`}>
            <EventRow event={e} />
          </li>
        ))}
      </ul>
      {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
    </>
  )
}

/** The consumption log, newest first, filterable by domain. */
export default function TasteEvents() {
  const [domain, setDomain] = useState('')
  return (
    <div className="space-y-3">
      <Field label="Domain" htmlFor="taste-domain" className="w-44">
        <Select id="taste-domain" value={domain} onChange={(e) => setDomain(e.target.value)}>
          <option value="">Everything</option>
          {DOMAINS.map((d) => (
            <option key={d.id} value={d.id}>
              {d.label}
            </option>
          ))}
        </Select>
      </Field>
      {/* Keyed so switching domains starts a fresh paginated list. */}
      <EventList key={domain} domain={domain} />
    </div>
  )
}

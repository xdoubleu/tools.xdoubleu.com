'use client'

import { mutate } from 'swr'
import { useBookSeries } from '@/hooks/useBookSeries'
import { type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Alert } from '@/components/ui/alert'
import { PageHeader } from '@/components/ui/page-header'
import { PageContainer } from '@/components/ui/page-container'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import SeriesEntryRow from '@/components/books/SeriesEntryRow'
import { swrKeys } from '@/lib/swrKeys'
import { seriesDescription } from '@/lib/books/series'

export default function SeriesBooksClient({ name }: { name: string }) {
  const { data, error, isLoading } = useBookSeries(name)

  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Books', href: '/dashboard/reading' },
    { label: 'Library', href: '/books/library' },
    { label: name }
  ]

  const handleAdded = () => {
    void mutate(swrKeys.bookSeries(name))
    void mutate(swrKeys.books)
  }

  return (
    <PageContainer className="space-y-4">
      <PageHeader
        breadcrumb={breadcrumbItems}
        title={name}
        description={data ? seriesDescription(data) : undefined}
      />

      {isLoading && <LoadingState label="series" className="text-sm" />}
      {error && <ErrorState what="series" className="text-sm" />}

      {data?.externalUnavailable && (
        <Alert tone="info">
          Couldn&apos;t check Hardcover, so missing volumes aren&apos;t shown.
        </Alert>
      )}

      {data && data.entries.length === 0 && <EmptyState>No books in this series yet.</EmptyState>}

      {data && data.entries.length > 0 && (
        <ol className="grid grid-cols-1 gap-3 sm:grid-cols-2" data-testid="series-entries">
          {data.entries.map((entry, i) => (
            // The server returns a stable order, so the index is a stable key.
            <li key={i}>
              <SeriesEntryRow entry={entry} onAdded={handleAdded} />
            </li>
          ))}
        </ol>
      )}
    </PageContainer>
  )
}

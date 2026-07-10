'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useResyncRefresh } from '@/lib/books/resyncRefresh'
import ManageDuplicatesDialog from '@/components/books/ManageDuplicatesDialog'
import ResyncWizard from '@/components/books/ResyncWizard'
import { Button } from '@/components/ui/button'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'
import { formatDateTime } from '@/lib/dates'

export default function BooksAdminClient() {
  const { isRefreshing, lastRefresh, processed, total, refresh } = useResyncRefresh(
    () => void mutate(swrKeys.resyncProposals)
  )

  const [duplicatesDialogOpen, setDuplicatesDialogOpen] = useState(false)

  return (
    <PageContainer className="max-w-2xl p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Books', href: '/books' }, { label: 'Admin tools' }]}
      />
      <h1 className="mb-6 text-3xl font-bold">Books admin tools</h1>

      {/* Find duplicates */}
      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Find duplicates
        </h2>
        <p className="mb-3 text-xs text-muted">
          Detect duplicate library entries and merge them. Matching is based on ISBN or normalised
          title and author. Files, tags, and reading progress are consolidated onto the entry you
          choose to keep.
        </p>
        <Button
          type="button"
          variant="secondary"
          onClick={() => setDuplicatesDialogOpen(true)}
          data-testid="find-duplicates-btn"
        >
          Find duplicates
        </Button>
      </section>

      {/* Scan for differences */}
      <section className="mt-10 border-t border-border pt-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Scan for metadata differences
        </h2>
        <p className="mb-3 text-xs text-muted">
          Fetch Open Library, Google Books, and UniCat for every book and flag any that differ from
          your library. Nothing is written automatically — review each flagged book below and pick
          which source (or your existing library value) should win.
        </p>
        <Button
          type="button"
          variant="secondary"
          disabled={isRefreshing}
          onClick={refresh}
          data-testid="resync-openlibrary-btn"
        >
          {isRefreshing ? 'Scanning…' : 'Start resync'}
        </Button>
        {isRefreshing && (
          <div className="mt-3" data-testid="resync-openlibrary-progress">
            {total !== null ? (
              <>
                <div className="mb-1 flex justify-between text-xs text-muted">
                  <span>Scanning books…</span>
                  <span>
                    {processed ?? 0} / {total}
                  </span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded-full bg-border">
                  <div
                    className="h-full rounded-full bg-fg transition-all duration-300"
                    style={{
                      width: `${total > 0 ? (((processed ?? 0) / total) * 100).toFixed(1) : 0}%`
                    }}
                  />
                </div>
              </>
            ) : (
              <p className="text-xs text-muted">Scanning…</p>
            )}
          </div>
        )}
        {!isRefreshing && lastRefresh && (
          <p className="mt-2 text-xs text-muted" data-testid="resync-openlibrary-status">
            Last scanned {formatDateTime(lastRefresh)}
          </p>
        )}
      </section>

      {/* Resync wizard */}
      <section className="mt-10 border-t border-border pt-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Review flagged books
        </h2>
        <p className="mb-4 text-xs text-muted">
          Step through books flagged by the last scan. For each one, pick the source you trust — or
          keep your library value — and apply.
        </p>
        <ResyncWizard />
      </section>

      <ManageDuplicatesDialog open={duplicatesDialogOpen} onOpenChange={setDuplicatesDialogOpen} />
    </PageContainer>
  )
}

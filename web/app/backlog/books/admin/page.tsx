'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useResyncRefresh } from '@/lib/backlog/resyncRefresh'
import ManageDuplicatesDialog from '@/components/backlog/ManageDuplicatesDialog'
import SelectiveResync from '@/components/backlog/SelectiveResync'
import { Button } from '@/components/ui/button'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogBooksAdminPage() {
  const { isRefreshing, lastRefresh, processed, total, refresh } = useResyncRefresh(
    () => void mutate('/backlog/books')
  )

  const [duplicatesDialogOpen, setDuplicatesDialogOpen] = useState(false)

  return (
    <main className="mx-auto max-w-2xl px-4 py-10">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Books', href: '/backlog/books' }, { label: 'Admin tools' }]}
      />
      <h1 className="mb-6 text-xl font-semibold text-fg">Books admin tools</h1>

      {/* Resync all */}
      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Resync all metadata
        </h2>
        <p className="mb-3 text-xs text-muted">
          Re-fetch covers, descriptions, and page counts for all books missing metadata. Books with
          an ISBN are looked up via Open Library then Google Books. ISBN-less books are matched by
          title and author — a confident match also fills in the ISBN so future resyncs are faster.
          Existing cached covers are cleared so updated images download on next view.
        </p>
        <Button
          type="button"
          variant="secondary"
          disabled={isRefreshing}
          onClick={refresh}
          data-testid="resync-openlibrary-btn"
        >
          {isRefreshing ? 'Resyncing…' : 'Resync all metadata'}
        </Button>
        {isRefreshing && (
          <div className="mt-3" data-testid="resync-openlibrary-progress">
            {total !== null ? (
              <>
                <div className="mb-1 flex justify-between text-xs text-muted">
                  <span>Resyncing books…</span>
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
              <p className="text-xs text-muted">Resyncing…</p>
            )}
          </div>
        )}
        {!isRefreshing && lastRefresh && (
          <p className="mt-2 text-xs text-muted" data-testid="resync-openlibrary-status">
            Last synced {lastRefresh.toLocaleString()}
          </p>
        )}
      </section>

      {/* Selective resync */}
      <section className="mt-10 border-t border-border pt-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Selective resync
        </h2>
        <p className="mb-4 text-xs text-muted">
          Pick individual books to resync. Use the filter chips to narrow the list to books with
          missing ISBNs or books that could not be found in Open Library or Google Books. Enable
          Force re-fetch to overwrite existing metadata (cover, description, page count) with fresh
          provider data.
        </p>
        <SelectiveResync />
      </section>

      {/* Find duplicates */}
      <section className="mt-10 border-t border-border pt-8">
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

      <ManageDuplicatesDialog open={duplicatesDialogOpen} onOpenChange={setDuplicatesDialogOpen} />
    </main>
  )
}

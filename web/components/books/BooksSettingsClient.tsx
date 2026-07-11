'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useImportBooks } from '@/hooks/useBooks'
import { useCurrentUser } from '@/hooks/useAuth'
import BulkBookUploader from '@/components/books/BulkBookUploader'
import KoboSetup from '@/components/books/KoboSetup'
import KoboDevices from '@/components/books/KoboDevices'
import ClearLibraryDialog from '@/components/books/ClearLibraryDialog'
import { mutate } from 'swr'
import { Button } from '@/components/ui/button'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'

export default function BooksSettingsClient() {
  const importBooks = useImportBooks()
  const { data: currentUser } = useCurrentUser()
  const isAdmin = currentUser?.role === 'admin'

  const [importStatus, setImportStatus] = useState('')
  const [clearDialogOpen, setClearDialogOpen] = useState(false)
  const [clearStatus, setClearStatus] = useState('')

  function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImportStatus('Importing…')
    const reader = new FileReader()
    reader.onload = async (ev) => {
      const csvData = ev.target?.result
      if (typeof csvData !== 'string') return
      try {
        const res = await importBooks(csvData)
        setImportStatus(`Imported ${res.importedCount} book(s).`)
        await mutate(swrKeys.books)
      } catch {
        setImportStatus('Import failed.')
      }
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  return (
    <PageContainer size="narrow" className="p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Books', href: '/books' }, { label: 'Settings' }]}
      />
      <h1 className="mb-6 text-3xl font-bold">Books Settings</h1>

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Import books
        </h2>
        <p className="mb-3 text-xs text-muted">
          Import your library from a Goodreads (or compatible) CSV export.
        </p>
        <div className="flex items-center gap-2">
          <label className="inline-flex h-9 cursor-pointer items-center rounded-xl border border-border bg-surface px-3 text-sm text-fg transition-colors hover:bg-hover">
            Import CSV
            <input type="file" accept=".csv" onChange={handleImport} className="hidden" />
          </label>
          {importStatus && <span className="text-sm text-muted">{importStatus}</span>}
        </div>
      </section>

      <section className="mt-10 border-t border-border pt-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Upload ebooks
        </h2>
        <p className="mb-3 text-xs text-muted">
          Upload EPUB or PDF files to your digital library. Books are auto-recognized and added as
          own-digital.
        </p>
        <BulkBookUploader />
      </section>

      <section className="mt-10 border-t border-border pt-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Kobo</h2>
        <p className="mb-3 text-xs text-muted">
          Connect your Kobo device for wireless sync via the kobo-gateway menu-bar app. Each device
          gets its own sync token; disconnecting a device immediately revokes its access.
        </p>
        <KoboSetup />

        <div className="mt-6">
          <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted">
            Connected devices
          </h3>
          <KoboDevices />
        </div>
      </section>

      {isAdmin && (
        <section className="mt-10 border-t border-border pt-8">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
            Admin tools
          </h2>
          <p className="mb-3 text-xs text-muted">
            Resync metadata, selectively re-fetch individual books, and merge duplicates.
          </p>
          <Button asChild variant="secondary">
            <Link href="/books/admin">Open admin tools</Link>
          </Button>
        </section>
      )}

      <section className="mt-10 border-t border-border pt-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Danger zone
        </h2>
        <p className="mb-3 text-xs text-muted">
          Permanently delete all books, reading progress, and uploaded files from your library. This
          cannot be undone.
        </p>
        <Button
          type="button"
          variant="destructive"
          onClick={() => {
            setClearStatus('')
            setClearDialogOpen(true)
          }}
          data-testid="clear-library-btn"
        >
          Clear library
        </Button>
        {clearStatus && (
          <p className="mt-2 text-sm text-success" data-testid="clear-library-status">
            {clearStatus}
          </p>
        )}
      </section>

      <ClearLibraryDialog
        open={clearDialogOpen}
        onOpenChange={setClearDialogOpen}
        onCleared={() => setClearStatus('Library cleared successfully.')}
      />
    </PageContainer>
  )
}

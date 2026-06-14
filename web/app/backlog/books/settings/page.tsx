'use client'

import { useState } from 'react'
import { useImportBooks } from '@/hooks/useBacklog'
import BulkBookUploader from '@/components/backlog/BulkBookUploader'
import KoboSetup from '@/components/backlog/KoboSetup'
import KoboDevices from '@/components/backlog/KoboDevices'
import ClearLibraryDialog from '@/components/backlog/ClearLibraryDialog'
import { mutate } from 'swr'
import { Button } from '@/components/ui/button'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogBooksSettingsPage() {
  const importBooks = useImportBooks()

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
        await mutate('/backlog/books')
      } catch {
        setImportStatus('Import failed.')
      }
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  return (
    <main className="mx-auto max-w-xl px-4 py-10">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Books', href: '/backlog/books' }, { label: 'Settings' }]}
      />
      <h1 className="mb-6 text-xl font-semibold text-fg">Books Settings</h1>

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Import books
        </h2>
        <p className="mb-3 text-xs text-muted">
          Import your library from a Goodreads or Hardcover CSV export.
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
          Connect your Kobo device for wireless sync. Plug it in, click Select my Kobo, and choose
          the Kobo drive in the picker — the app then configures it. Each device gets its own sync
          token; disconnecting a device immediately revokes its access.
        </p>
        <KoboSetup />

        <div className="mt-6">
          <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted">
            Connected devices
          </h3>
          <KoboDevices />
        </div>
      </section>

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
    </main>
  )
}

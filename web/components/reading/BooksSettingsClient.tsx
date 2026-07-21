'use client'

import { useState } from 'react'
import { useImportBooks } from '@/hooks/useBooks'
import BulkBookUploader from '@/components/reading/BulkBookUploader'
import FeedManager from '@/components/reading/FeedManager'
import KoboSetup from '@/components/reading/KoboSetup'
import KoboDevices from '@/components/reading/KoboDevices'
import { mutate } from 'swr'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'

export default function BooksSettingsClient() {
  const importBooks = useImportBooks()

  const [importStatus, setImportStatus] = useState('')

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
        items={[{ label: 'Reading', href: '/reading' }, { label: 'Settings' }]}
      />
      <h1 className="mb-6 text-3xl font-bold">Reading Settings</h1>

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
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">RSS feeds</h2>
        <p className="mb-3 text-xs text-muted">
          Subscribe to blogs and news feeds. New posts are converted to EPUB and added to your
          library; feeds with Kobo sync enabled push every new post to your Kobo automatically.
        </p>
        <FeedManager />
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
    </PageContainer>
  )
}

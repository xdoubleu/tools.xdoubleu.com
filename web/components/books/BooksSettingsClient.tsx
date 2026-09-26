'use client'

import { useRef, useState } from 'react'
import { useImportBooks } from '@/hooks/useBooks'
import BulkBookUploader from '@/components/books/BulkBookUploader'
import KoboSetup from '@/components/books/KoboSetup'
import KoboDevices from '@/components/books/KoboDevices'
import { mutate } from 'swr'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/ui/page-header'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'

export default function BooksSettingsClient() {
  const importBooks = useImportBooks()

  const [importStatus, setImportStatus] = useState('')
  const csvInputRef = useRef<HTMLInputElement>(null)

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
    <PageContainer size="narrow">
      <PageHeader
        breadcrumb={[{ label: 'Reading', href: '/dashboard/reading' }, { label: 'Settings' }]}
        title="Reading Settings"
      />

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Import books
        </h2>
        <p className="mb-3 text-xs text-muted">
          Import your library from a Goodreads (or compatible) CSV export.
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => csvInputRef.current?.click()}
          >
            Import CSV
          </Button>
          {/* eslint-disable-next-line no-restricted-syntax -- a file picker must be a real hidden <input type="file">; no primitive can wrap it */}
          <input
            ref={csvInputRef}
            type="file"
            accept=".csv"
            onChange={handleImport}
            className="hidden"
            data-testid="csv-input"
          />
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
    </PageContainer>
  )
}

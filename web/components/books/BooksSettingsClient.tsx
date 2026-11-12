import { useRef, useState } from 'react'
import { useImportBooks } from '@/hooks/useBooks'
import BulkBookUploader from '@/components/books/BulkBookUploader'
import KoboSetup from '@/components/books/KoboSetup'
import KoboDevices from '@/components/books/KoboDevices'
import OfflineBooksSettings from '@/components/books/OfflineBooksSettings'
import { mutate } from 'swr'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/ui/page-header'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'
import { SectionHeading } from '@/components/ui/section-heading'
import { SettingsSection, SettingsSections } from '@/components/ui/settings-section'

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
        breadcrumb={[{ label: 'Books', href: '/books/library' }, { label: 'Settings' }]}
        title="Books settings"
      />

      <SettingsSections>
        <SettingsSection
          title="Import books"
          description="Import your library from a Goodreads (or compatible) CSV export."
        >
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" variant="secondary" onClick={() => csvInputRef.current?.click()}>
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
            {importStatus && (
              <span className="text-sm text-muted" role="status">
                {importStatus}
              </span>
            )}
          </div>
        </SettingsSection>

        <SettingsSection
          title="Upload ebooks"
          description="Upload EPUB or PDF files to your digital library. Books are auto-recognized and added as own-digital."
        >
          <BulkBookUploader />
        </SettingsSection>

        <SettingsSection
          title="Kobo"
          description="Connect your Kobo device for wireless sync via the kobo-gateway menu-bar app. Each device gets its own sync token; disconnecting a device immediately revokes its access."
        >
          <KoboSetup />
          <div className="pt-3">
            <SectionHeading level="eyebrow" className="mb-3">
              Connected devices
            </SectionHeading>
            <KoboDevices />
          </div>
        </SettingsSection>

        <OfflineBooksSettings />
      </SettingsSections>
    </PageContainer>
  )
}

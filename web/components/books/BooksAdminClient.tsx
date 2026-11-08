import { useState } from 'react'
import { mutate } from 'swr'
import { useCancelResync } from '@/hooks/useBooks'
import { useResyncRefresh } from '@/lib/books/resyncRefresh'
import ManageDuplicatesDialog from '@/components/books/ManageDuplicatesDialog'
import ResyncWizard from '@/components/books/ResyncWizard'
import SourceStats from '@/components/books/SourceStats'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { PageHeader } from '@/components/ui/page-header'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'
import { ProgressBar } from '@/components/ui/progress-bar'
import { SettingsSection, SettingsSections } from '@/components/ui/settings-section'
import { LoadingState } from '@/components/ui/states'
import { formatDateTime } from '@/lib/dates'

export default function BooksAdminClient() {
  const [force, setForce] = useState(false)
  const { isRefreshing, lastRefresh, processed, total, refresh } = useResyncRefresh(() => {
    void mutate(swrKeys.resyncProposals)
    void mutate(swrKeys.bookSourceStats)
  }, force)
  const cancelResync = useCancelResync()

  const [duplicatesDialogOpen, setDuplicatesDialogOpen] = useState(false)

  return (
    <PageContainer size="form">
      <PageHeader
        breadcrumb={[{ label: 'Books', href: '/dashboard/reading' }, { label: 'Admin tools' }]}
        title="Books admin tools"
      />

      <SettingsSections>
        <SettingsSection
          title="Find duplicates"
          description="Detect duplicate library entries and merge them. Matching is based on ISBN or normalised title and author. Files, tags, and reading progress are consolidated onto the entry you choose to keep."
        >
          <Button
            type="button"
            variant="secondary"
            onClick={() => setDuplicatesDialogOpen(true)}
            data-testid="find-duplicates-btn"
          >
            Find duplicates
          </Button>
        </SettingsSection>

        <SettingsSection
          title="Scan for metadata differences"
          description="Fetch UniCat and Hardcover for every book and flag any that differ from your library. Nothing is written automatically — review each flagged book below and pick which source (or your existing library value) should win."
        >
          <Checkbox
            id="resync-force"
            checked={force}
            disabled={isRefreshing}
            onChange={(e) => setForce(e.target.checked)}
            label="Force re-check all sources (ignore cache)"
            data-testid="resync-force-checkbox"
          />
          <div className="flex gap-2">
            <Button
              type="button"
              variant="secondary"
              disabled={isRefreshing}
              onClick={refresh}
              data-testid="resync-books-btn"
            >
              {isRefreshing ? 'Scanning…' : 'Start resync'}
            </Button>
            {isRefreshing && (
              <Button
                type="button"
                variant="destructive"
                onClick={() => void cancelResync()}
                data-testid="resync-cancel-btn"
              >
                Stop
              </Button>
            )}
          </div>
          {isRefreshing && (
            <div data-testid="resync-books-progress">
              {total !== null ? (
                <>
                  <div className="mb-1 flex justify-between text-xs text-muted">
                    <span>Scanning books…</span>
                    <span>
                      {processed ?? 0} / {total}
                    </span>
                  </div>
                  <ProgressBar
                    value={processed ?? 0}
                    max={total}
                    aria-label="Resync progress"
                    valueText={`${processed ?? 0} of ${total} books`}
                  />
                </>
              ) : (
                <LoadingState />
              )}
            </div>
          )}
          {!isRefreshing && lastRefresh && (
            <p className="text-xs text-muted" data-testid="resync-books-status">
              Last scanned {formatDateTime(lastRefresh)}
            </p>
          )}
        </SettingsSection>

        <SettingsSection
          title="Review flagged books"
          description="Step through books flagged by the last scan. For each one, pick the source you trust — or keep your library value — and apply."
        >
          <ResyncWizard />
        </SettingsSection>

        <SettingsSection
          title="Source stats"
          description="Found counts how many books the last scan located in each source; Applied counts how many books currently use that source’s metadata (recorded whenever you apply one)."
        >
          <SourceStats />
        </SettingsSection>
      </SettingsSections>

      <ManageDuplicatesDialog open={duplicatesDialogOpen} onOpenChange={setDuplicatesDialogOpen} />
    </PageContainer>
  )
}

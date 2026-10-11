'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useEnableKoboSync, useSetBookTag, useKEPUBStatus } from '@/hooks/useBooks'
import { swrKeys } from '@/lib/swrKeys'
import { Checkbox } from '@/components/ui/checkbox'
import { cn } from '@/lib/cn'

interface KoboSyncToggleProps {
  bookId: string
  enabled: boolean
  onChanged?: () => void
}

function kepubStatusLabel(status: string): string {
  if (status === 'converting') return 'Preparing for Kobo…'
  if (status === 'ready') return 'Ready to sync'
  if (status === 'failed') return 'Conversion failed'
  return ''
}

export default function KoboSyncToggle({ bookId, enabled, onChanged }: KoboSyncToggleProps) {
  const [enabledState, setEnabledState] = useState(enabled)
  const [toggling, setToggling] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const enableKoboSync = useEnableKoboSync()
  const setBookTag = useSetBookTag()

  const { data: statusData } = useKEPUBStatus(bookId)

  const hasEpub = statusData?.hasEpub ?? false
  const hasPdf = statusData?.hasPdf ?? false
  const kepubStatus = statusData?.kepubStatus ?? ''

  const canEnable = hasEpub || hasPdf

  const handleToggle = async () => {
    const wasEnabled = enabledState
    setEnabledState(!wasEnabled)
    setToggling(true)
    setError(null)
    try {
      if (wasEnabled) {
        await setBookTag(bookId, 'kobo-sync', false)
      } else {
        await enableKoboSync(bookId)
        mutate(swrKeys.kepubStatus(bookId))
      }
      onChanged?.()
    } catch (err) {
      setEnabledState(wasEnabled)
      setError(err instanceof Error ? err.message : 'Failed to update Kobo sync.')
    } finally {
      setToggling(false)
    }
  }

  return (
    <div className="space-y-2">
      <Checkbox
        id="kobo-sync-toggle"
        checked={enabledState}
        disabled={(!canEnable && !enabledState) || toggling}
        onChange={handleToggle}
        data-testid="kobo-sync-checkbox"
        label={<span className="text-sm text-subtle">Kobo sync</span>}
      />

      {!canEnable && !enabledState && (
        <p className="text-xs text-muted">Upload an EPUB or PDF to enable Kobo sync.</p>
      )}

      {enabledState && kepubStatus && (
        <p
          className={cn('text-xs', kepubStatus === 'failed' ? 'text-danger' : 'text-muted')}
          data-testid="kepub-status"
        >
          {kepubStatusLabel(kepubStatus)}
        </p>
      )}

      {error && <p className="text-xs text-danger">{error}</p>}
    </div>
  )
}

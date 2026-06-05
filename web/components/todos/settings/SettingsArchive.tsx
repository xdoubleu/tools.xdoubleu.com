'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface Props {
  data: GetSettingsResponse
  mutate: () => void
}

export function SettingsArchive({ data, mutate }: Props) {
  const current = data.archive?.archiveAfterHours ?? 0
  const [hours, setHours] = useState(current)
  const [saved, setSaved] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const client = createServiceClient(SettingsService)
    await client.updateArchiveSettings({ archiveAfterHours: hours })
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
    mutate()
  }

  return (
    <section aria-labelledby="archive-heading">
      <h2
        id="archive-heading"
        className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
      >
        Archive Settings
      </h2>
      <form onSubmit={handleSubmit} className="flex items-center gap-3">
        <label className="text-sm text-subtle">Archive completed tasks after</label>
        <Input
          type="number"
          min={0}
          value={hours}
          onChange={(e) => {
            setHours(Number(e.target.value))
            setSaved(false)
          }}
          className="h-9 w-20"
        />
        <label className="text-sm text-subtle">hour{hours === 1 ? '' : 's'}</label>
        <Button type="submit" size="sm" disabled={hours === current}>
          Save
        </Button>
        {saved && <span className="text-xs text-muted">Saved</span>}
      </form>
    </section>
  )
}

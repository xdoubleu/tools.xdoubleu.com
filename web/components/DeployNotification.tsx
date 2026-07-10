'use client'

import { useRef, useState } from 'react'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { swrKeys } from '@/lib/swrKeys'
import { getRelease } from '@/lib/env'

const POLL_INTERVAL_MS = 60_000

async function fetchRelease(url: string): Promise<{ release: string }> {
  const res = await fetch(url)
  return res.json()
}

export default function DeployNotification() {
  const baseline = useRef(getRelease())
  const [dismissed, setDismissed] = useState(false)
  const { data } = useSWR(swrKeys.webRelease, fetchRelease, {
    refreshInterval: POLL_INTERVAL_MS
  })

  // ponytail: 'dev' is the local/unset baseline, never treat it as a real
  // deploy so this never fires outside a deployed environment.
  const newVersionAvailable =
    !dismissed && baseline.current !== 'dev' && !!data?.release && data.release !== baseline.current

  if (!newVersionAvailable) return null

  return (
    <div className="fixed bottom-4 right-4 z-50 max-w-xs">
      <Card className="flex items-center gap-3 p-4">
        <p className="flex-1 text-sm text-fg">A new version is available.</p>
        <Button size="sm" onClick={() => window.location.reload()}>
          Reload
        </Button>
        <Button
          variant="ghost"
          size="iconSm"
          aria-label="Dismiss"
          onClick={() => setDismissed(true)}
        >
          ×
        </Button>
      </Card>
    </div>
  )
}

'use client'

import { useEffect, useState, useSyncExternalStore } from 'react'
import { Alert } from '@/components/ui/alert'
import { formatAge } from '@/lib/offline/formatAge'
import { getServerSnapshot, getSnapshot, subscribe } from '@/lib/offline/status'

const TICK_MS = 30_000

export default function OfflineBanner() {
  const { offline, servedFromCacheAt } = useSyncExternalStore(
    subscribe,
    getSnapshot,
    getServerSnapshot
  )
  const [age, setAge] = useState('')

  useEffect(() => {
    if (servedFromCacheAt === null) return
    const update = () => setAge(formatAge(Date.now() - servedFromCacheAt))
    update()
    const id = setInterval(update, TICK_MS)
    return () => clearInterval(id)
  }, [servedFromCacheAt])

  if (!offline && servedFromCacheAt === null) return null

  return (
    <Alert tone="warn" className="mb-4">
      {offline ? 'Offline' : 'Can’t reach the server'}
      {servedFromCacheAt !== null && ` · showing data saved ${age}`}
    </Alert>
  )
}

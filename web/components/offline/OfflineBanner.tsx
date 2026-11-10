import { useEffect, useState, useSyncExternalStore } from 'react'
import { Link } from '@/components/ui/link'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { formatAge } from '@/lib/offline/formatAge'
import {
  dismissFailed,
  getOutboxServerSnapshot,
  getOutboxSnapshot,
  subscribeOutbox
} from '@/lib/offline/outbox'
import { getServerSnapshot, getSnapshot, subscribe } from '@/lib/offline/status'

const TICK_MS = 30_000
// A write is queued before it is sent, so online it is pending for a moment; only a
// write that stays pending is worth a warning.
const PENDING_GRACE_MS = 2_000

const changes = (n: number) => `${n} ${n === 1 ? 'change' : 'changes'}`

export default function OfflineBanner() {
  const { offline, servedFromCacheAt } = useSyncExternalStore(
    subscribe,
    getSnapshot,
    getServerSnapshot
  )
  const { pending, failed, authBlocked } = useSyncExternalStore(
    subscribeOutbox,
    getOutboxSnapshot,
    getOutboxServerSnapshot
  )
  const [age, setAge] = useState('')
  const hasPending = pending > 0
  const [pendingLingers, setPendingLingers] = useState(false)

  useEffect(() => {
    if (!hasPending) {
      setPendingLingers(false)
      return
    }
    const id = setTimeout(() => setPendingLingers(true), PENDING_GRACE_MS)
    return () => clearTimeout(id)
  }, [hasPending])

  useEffect(() => {
    if (servedFromCacheAt === null) return
    const update = () => setAge(formatAge(Date.now() - servedFromCacheAt))
    update()
    const id = setInterval(update, TICK_MS)
    return () => clearInterval(id)
  }, [servedFromCacheAt])

  const status = [
    offline ? 'Offline' : servedFromCacheAt !== null && 'Can’t reach the server',
    pendingLingers && !authBlocked && `${changes(pending)} waiting to sync`,
    servedFromCacheAt !== null && `showing data saved ${age}`
  ].filter(Boolean)

  return (
    <>
      {authBlocked && (
        <Alert tone="warn" className="mb-4">
          Your session expired. <Link href="/auth/sign-in">Sign in again</Link> to sync{' '}
          {changes(pending)}.
        </Alert>
      )}
      {status.length > 0 && (
        <Alert tone="warn" className="mb-4">
          {status.join(' · ')}
        </Alert>
      )}
      {failed.length > 0 && (
        <Alert tone="danger" className="mb-4 flex items-start gap-3">
          <div className="flex-1">
            <p>Couldn’t sync {changes(failed.length)}:</p>
            <ul className="list-disc pl-5">
              {failed.map((f, i) => (
                <li key={i}>
                  {f.description} ({f.reason})
                </li>
              ))}
            </ul>
          </div>
          <Button variant="ghost" size="sm" onClick={() => void dismissFailed()}>
            Dismiss
          </Button>
        </Alert>
      )}
    </>
  )
}

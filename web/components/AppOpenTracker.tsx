import { useEffect, useRef } from 'react'
import { usePathname } from '@/hooks/navigation'
import { useCurrentUser, useRecordAppOpen } from '@/hooks/useAuth'
import { appForPath, pinnableApps } from '@/lib/apps'

/**
 * Tells the api when the signed-in user switches to an app, which only ranks the
 * default tab bar pins of someone who never chose. Renders nothing; failures are
 * dropped, so offline use is unaffected.
 */
export default function AppOpenTracker() {
  const { data } = useCurrentUser()
  const recordAppOpen = useRecordAppOpen()
  const last = useRef<string | null>(null)
  const app = appForPath(usePathname())
  const id = data && app && pinnableApps(data).some((a) => a.id === app.id) ? app.id : null

  useEffect(() => {
    // Stryker disable next-line ConditionalExpression,EqualityOperator,LogicalOperator: the effect reruns only when id changes; the repeat check guards StrictMode's double run.
    if (id === null || id === last.current) return
    last.current = id
    recordAppOpen(id).catch(() => {})
  }, [id])

  return null
}

import { useEffect } from 'react'
import OfflineBooksSync from '@/components/books/OfflineBooksSync'
import DeployNotification from '@/components/DeployNotification'
import AppOpenTracker from '@/components/AppOpenTracker'
import Navbar from '@/components/Navbar'
import PhoneTabBar, { useTabBarVisible } from '@/components/PhoneTabBar'
import OfflineBanner from '@/components/offline/OfflineBanner'
import OutboxSync from '@/components/offline/OutboxSync'
import ServiceWorkerRegistrar from '@/components/offline/ServiceWorkerRegistrar'
import Sidebar from '@/components/Sidebar'
import SWRProvider, { useSessionEffects } from '@/components/SWRProvider'
import { usePathname, useRouter, useSearchParams } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'
import { cn } from '@/lib/cn'
import { isSharedDashboard, signInRedirect } from '@/lib/signIn'

// Sends a visitor without a session to sign-in, returning here after. The page
// isn't rendered meanwhile, so it can't flash its "Failed to load" state.
function useSignInRedirect(error: unknown): boolean {
  const router = useRouter()
  const href = signInRedirect(error, usePathname(), useSearchParams().toString())
  useEffect(() => {
    if (href) router.replace(href)
  }, [href, router])
  return href !== null
}

// Stryker disable StringLiteral: layout classes; the sidebar offset is tested.
function Shell({ children }: { children: React.ReactNode }) {
  const { data: currentUser, error } = useCurrentUser()
  useSessionEffects(currentUser?.userId)
  const pathname = usePathname()
  const tabBar = useTabBarVisible()
  const redirecting = useSignInRedirect(error)

  const booksAccess =
    currentUser !== undefined &&
    (currentUser.role === 'admin' || currentUser.appAccess.includes('books'))

  const signedIn = currentUser !== undefined && !isSharedDashboard(pathname)

  return (
    <>
      <Sidebar />
      <div className={cn('flex flex-1 flex-col', signedIn && 'lg:pl-60')}>
        <Navbar />
        <main
          className={cn(
            'flex-1 pl-[var(--inset-left)] pr-[var(--inset-right)] py-6',
            tabBar && 'pb-[calc(var(--tab-bar-height)+env(safe-area-inset-bottom)+1.5rem)] lg:pb-6'
          )}
        >
          <div className="w-full">
            <OfflineBanner />
            {!redirecting && children}
          </div>
        </main>
      </div>
      <PhoneTabBar />
      <AppOpenTracker />
      <DeployNotification />
      <ServiceWorkerRegistrar />
      <OutboxSync />
      {booksAccess && <OfflineBooksSync />}
    </>
  )
}

// Stryker restore StringLiteral

export default function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <SWRProvider>
      <Shell>{children}</Shell>
    </SWRProvider>
  )
}

import { useEffect } from 'react'
import OfflineBooksSync from '@/components/books/OfflineBooksSync'
import DeployNotification from '@/components/DeployNotification'
import Footer from '@/components/Footer'
import Navbar from '@/components/Navbar'
import OfflineBanner from '@/components/offline/OfflineBanner'
import OutboxSync from '@/components/offline/OutboxSync'
import ServiceWorkerRegistrar from '@/components/offline/ServiceWorkerRegistrar'
import SWRProvider, { useSessionEffects } from '@/components/SWRProvider'
import { usePathname, useRouter, useSearchParams } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'
import { signInRedirect } from '@/lib/signIn'

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

function Shell({ children }: { children: React.ReactNode }) {
  const { data: currentUser, error } = useCurrentUser()
  useSessionEffects(currentUser?.userId)
  const redirecting = useSignInRedirect(error)

  const booksAccess =
    currentUser !== undefined &&
    (currentUser.role === 'admin' || currentUser.appAccess.includes('books'))

  return (
    <>
      <Navbar />
      <main className="flex-1 pl-[var(--inset-left)] pr-[var(--inset-right)] py-6">
        <div className="w-full">
          <OfflineBanner />
          {!redirecting && children}
        </div>
      </main>
      <Footer />
      <DeployNotification />
      <ServiceWorkerRegistrar />
      <OutboxSync />
      {booksAccess && <OfflineBooksSync />}
    </>
  )
}

export default function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <SWRProvider>
      <Shell>{children}</Shell>
    </SWRProvider>
  )
}

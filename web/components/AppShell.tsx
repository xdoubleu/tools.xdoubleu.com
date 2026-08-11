import DeployNotification from '@/components/DeployNotification'
import Footer from '@/components/Footer'
import Navbar from '@/components/Navbar'
import SWRProvider from '@/components/SWRProvider'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { AuthService } from '@/lib/gen/auth/v1/auth_pb'

// Split out of RootLayout so the current-user fetch (and everything below
// it) can be streamed in behind a Suspense boundary — the <html>/<body>
// shell reaches the browser immediately instead of waiting on this fetch.
export default async function AppShell({ children }: { children: React.ReactNode }) {
  const currentUser = await fetchOrNull(async () =>
    (await createServerClient(AuthService)).getCurrentUser({})
  )

  return (
    <SWRProvider currentUser={currentUser}>
      <Navbar />
      <main className="flex-1 px-4 py-6 sm:px-6 lg:px-10">
        <div className="w-full">{children}</div>
      </main>
      <Footer />
      <DeployNotification />
    </SWRProvider>
  )
}

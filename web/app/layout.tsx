import type { Metadata } from 'next'
import './globals.css'
import DeployNotification from '@/components/DeployNotification'
import Footer from '@/components/Footer'
import Navbar from '@/components/Navbar'
import SWRProvider from '@/components/SWRProvider'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { AuthService } from '@/lib/gen/auth/v1/auth_pb'

export const dynamic = 'force-dynamic'

export const metadata: Metadata = {
  title: 'tools.xdoubleu.com',
  description: 'Personal tools suite',
  appleWebApp: {
    capable: true,
    title: 'tools.xdoubleu.com',
    statusBarStyle: 'black-translucent'
  }
}

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const currentUser = await fetchOrNull(async () =>
    (await createServerClient(AuthService)).getCurrentUser({})
  )

  return (
    <html lang="en">
      <head>
        <meta
          name="viewport"
          content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no, viewport-fit=cover"
        />
        <meta name="msapplication-TileColor" content="#7c3aed" />
        <meta name="msapplication-TileImage" content="/apple-icon.png" />
        <link rel="mask-icon" href="/icon.svg" color="#7c3aed" />
        <script
          dangerouslySetInnerHTML={{
            __html: `window.__ENV__=${JSON.stringify({ API_URL: process.env.API_URL ?? '', SENTRY_DSN: process.env.SENTRY_DSN ?? '', RELEASE: process.env.RELEASE ?? 'dev' })}`
          }}
        />
        <script
          dangerouslySetInnerHTML={{
            __html: `document.addEventListener('gesturestart',function(e){e.preventDefault()});document.addEventListener('gesturechange',function(e){e.preventDefault()});`
          }}
        />
      </head>
      <body className="flex min-h-screen flex-col bg-bg text-fg">
        <SWRProvider currentUser={currentUser}>
          <Navbar />
          <main className="flex-1 px-4 py-6 sm:px-6">
            <div className="w-full">{children}</div>
          </main>
          <Footer />
          <DeployNotification />
        </SWRProvider>
      </body>
    </html>
  )
}

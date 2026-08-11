import { Suspense } from 'react'
import type { Metadata } from 'next'
import './globals.css'
import AppShell from '@/components/AppShell'
import Splash from '@/components/Splash'
import { themeInitScript } from '@/lib/theme'

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

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    // themeInitScript writes data-theme on <html> before hydration
    <html lang="en" suppressHydrationWarning>
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
            __html: `window.__ENV__=${JSON.stringify({ API_URL: process.env.API_URL ?? '', SENTRY_DSN: process.env.SENTRY_DSN ?? '', RELEASE: process.env.RELEASE ?? 'dev', KOBO_GATEWAY_RELEASE: process.env.KOBO_GATEWAY_RELEASE ?? 'dev' })}`
          }}
        />
        <script dangerouslySetInnerHTML={{ __html: themeInitScript }} />
        <script
          dangerouslySetInnerHTML={{
            __html: `document.addEventListener('gesturestart',function(e){e.preventDefault()});document.addEventListener('gesturechange',function(e){e.preventDefault()});`
          }}
        />
      </head>
      <body className="flex min-h-screen flex-col bg-bg text-fg">
        <Suspense fallback={<Splash />}>
          <AppShell>{children}</AppShell>
        </Suspense>
      </body>
    </html>
  )
}
